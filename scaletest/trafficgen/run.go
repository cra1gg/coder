package trafficgen

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/xerrors"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/sloghuman"

	"github.com/coder/coder/coderd/tracing"
	"github.com/coder/coder/codersdk"
	"github.com/coder/coder/cryptorand"
	"github.com/coder/coder/scaletest/harness"
	"github.com/coder/coder/scaletest/loadtestutil"
)

type Runner struct {
	client *codersdk.Client
	cfg    Config
}

var (
	_ harness.Runnable  = &Runner{}
	_ harness.Cleanable = &Runner{}
)

func NewRunner(client *codersdk.Client, cfg Config) *Runner {
	return &Runner{
		client: client,
		cfg:    cfg,
	}
}

func (r *Runner) Run(ctx context.Context, _ string, logs io.Writer) error {
	ctx, span := tracing.StartSpan(ctx)
	defer span.End()

	logs = loadtestutil.NewSyncWriter(logs)
	logger := slog.Make(sloghuman.Sink(logs)).Leveled(slog.LevelDebug)
	r.client.Logger = logger
	r.client.LogBodies = true

	var (
		agentID             = r.cfg.AgentID
		reconnect           = uuid.New()
		height       uint16 = 65535
		width        uint16 = 65535
		tickInterval        = r.cfg.TicksPerSecond
		bytesPerTick        = r.cfg.BytesPerSecond / r.cfg.TicksPerSecond
	)

	logger.Debug(ctx, "connect to workspace agent", slog.F("agent_id", agentID))
	conn, err := r.client.WorkspaceAgentReconnectingPTY(ctx, codersdk.WorkspaceAgentReconnectingPTYOpts{
		AgentID:   agentID,
		Reconnect: reconnect,
		Height:    height,
		Width:     width,
		Command:   "/bin/sh",
	})
	if err != nil {
		logger.Error(ctx, "connect to workspace agent", slog.F("agent_id", agentID), slog.Error(err))
		return xerrors.Errorf("connect to workspace: %w", err)
	}

	defer func() {
		logger.Debug(ctx, "close agent connection", slog.F("agent_id", agentID))
		_ = conn.Close()
	}()

	// Set a deadline for stopping the text.
	start := time.Now()
	deadlineCtx, cancel := context.WithDeadline(ctx, start.Add(r.cfg.Duration))
	defer cancel()

	// Wrap the conn in a countReadWriter so we can monitor bytes sent/rcvd.
	crw := countReadWriter{ReadWriter: conn, ctx: deadlineCtx}

	// Create a ticker for sending data to the PTY.
	tick := time.NewTicker(time.Duration(tickInterval))
	defer tick.Stop()

	// Now we begin writing random data to the pty.
	rch := make(chan error)
	wch := make(chan error)

	go func() {
		<-deadlineCtx.Done()
		logger.Debug(ctx, "context deadline reached", slog.F("duration", time.Since(start)))
	}()

	// Read forever in the background.
	go func() {
		logger.Debug(ctx, "reading from agent", slog.F("agent_id", agentID))
		rch <- drainContext(deadlineCtx, &crw, bytesPerTick*2)
		logger.Debug(ctx, "done reading from agent", slog.F("agent_id", agentID))
		conn.Close()
		close(rch)
	}()

	// Write random data to the PTY every tick.
	go func() {
		logger.Debug(ctx, "writing to agent", slog.F("agent_id", agentID))
		wch <- writeRandomData(deadlineCtx, &crw, bytesPerTick, tick.C)
		logger.Debug(ctx, "done writing to agent", slog.F("agent_id", agentID))
		close(wch)
	}()

	// Wait for both our reads and writes to be finished.
	if wErr := <-wch; wErr != nil {
		return xerrors.Errorf("write to pty: %w", wErr)
	}
	if rErr := <-rch; rErr != nil {
		return xerrors.Errorf("read from pty: %w", rErr)
	}

	duration := time.Since(start)

	logger.Info(ctx, "results",
		slog.F("duration", duration),
		slog.F("sent", crw.BytesWritten()),
		slog.F("rcvd", crw.BytesRead()),
	)

	return nil
}

// Cleanup does nothing, successfully.
func (*Runner) Cleanup(context.Context, string) error {
	return nil
}

// drainContext drains from src until it returns io.EOF or ctx times out.
func drainContext(ctx context.Context, src io.Reader, bufSize int64) error {
	errCh := make(chan error)
	done := make(chan struct{})
	go func() {
		tmp := make([]byte, bufSize)
		buf := bytes.NewBuffer(tmp)
		for {
			select {
			case <-done:
				return
			default:
				_, err := io.CopyN(buf, src, 1)
				if err != nil {
					errCh <- err
					close(errCh)
					return
				}
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			close(done)
			return nil
		case err := <-errCh:
			if err != nil {
				if xerrors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}
}

func writeRandomData(ctx context.Context, dst io.Writer, size int64, tick <-chan time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick:
			payload := "#" + mustRandStr(size-1)
			data, err := json.Marshal(codersdk.ReconnectingPTYRequest{
				Data: payload,
			})
			if err != nil {
				return err
			}
			if _, err := copyContext(ctx, dst, data); err != nil {
				return err
			}
		}
	}
}

// copyContext copies from src to dst until ctx is canceled.
func copyContext(ctx context.Context, dst io.Writer, src []byte) (int, error) {
	var count int
	for {
		select {
		case <-ctx.Done():
			return count, nil
		default:
			for idx := range src {
				n, err := dst.Write(src[idx : idx+1])
				if err != nil {
					if xerrors.Is(err, io.EOF) {
						return count, nil
					}
					if xerrors.Is(err, context.DeadlineExceeded) {
						// It's OK if we reach the deadline before writing the full payload.
						return count, nil
					}
					return count, err
				}
				count += n
			}
			return count, nil
		}
	}
}

// countReadWriter wraps an io.ReadWriter and counts the number of bytes read and written.
type countReadWriter struct {
	ctx context.Context
	io.ReadWriter
	bytesRead    atomic.Int64
	bytesWritten atomic.Int64
}

func (w *countReadWriter) Read(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.ReadWriter.Read(p)
	if err == nil {
		w.bytesRead.Add(int64(n))
	}
	return n, err
}

func (w *countReadWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.ReadWriter.Write(p)
	if err == nil {
		w.bytesWritten.Add(int64(n))
	}
	return n, err
}

func (w *countReadWriter) BytesRead() int64 {
	return w.bytesRead.Load()
}

func (w *countReadWriter) BytesWritten() int64 {
	return w.bytesWritten.Load()
}

func mustRandStr(len int64) string {
	randStr, err := cryptorand.String(int(len))
	if err != nil {
		panic(err)
	}
	return randStr
}
