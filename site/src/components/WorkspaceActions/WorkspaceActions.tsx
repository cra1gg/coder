import { DropdownButton } from "components/DropdownButton/DropdownButton"
import { FC, ReactNode, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { WorkspaceStatus } from "../../api/typesGenerated"
import {
  ActionLoadingButton,
  ChangeVersionButton,
  DeleteButton,
  DisabledButton,
  SettingsButton,
  StartButton,
  StopButton,
  RestartButton,
  UpdateButton,
} from "../DropdownButton/ActionCtas"
import { ButtonMapping, ButtonTypesEnum, buttonAbilities } from "./constants"

export interface WorkspaceActionsProps {
  workspaceStatus: WorkspaceStatus
  isOutdated: boolean
  handleStart: () => void
  handleStop: () => void
  handleRestart: () => void
  handleDelete: () => void
  handleUpdate: () => void
  handleCancel: () => void
  handleSettings: () => void
  handleChangeVersion: () => void
  isUpdating: boolean
  children?: ReactNode
  canChangeVersions: boolean
}

export const WorkspaceActions: FC<WorkspaceActionsProps> = ({
  workspaceStatus,
  isOutdated,
  handleStart,
  handleStop,
  handleRestart,
  handleDelete,
  handleUpdate,
  handleCancel,
  handleSettings,
  handleChangeVersion,
  isUpdating,
  canChangeVersions,
}) => {
  const { t } = useTranslation("workspacePage")
  const { canCancel, canAcceptJobs, actions } = buttonAbilities(workspaceStatus)
  const canBeUpdated = isOutdated && canAcceptJobs

  // A mapping of button type to the corresponding React component
  const buttonMapping: ButtonMapping = {
    [ButtonTypesEnum.update]: <UpdateButton handleAction={handleUpdate} />,
    [ButtonTypesEnum.updating]: (
      <ActionLoadingButton label={t("actionButton.updating")} />
    ),
    [ButtonTypesEnum.settings]: (
      <SettingsButton handleAction={handleSettings} />
    ),
    [ButtonTypesEnum.changeVersion]: canChangeVersions ? (
      <ChangeVersionButton handleAction={handleChangeVersion} />
    ) : (
      <></>
    ),
    [ButtonTypesEnum.start]: <StartButton handleAction={handleStart} />,
    [ButtonTypesEnum.starting]: (
      <ActionLoadingButton label={t("actionButton.starting")} />
    ),
    [ButtonTypesEnum.stop]: <StopButton handleAction={handleStop} />,
    [ButtonTypesEnum.stopping]: (
      <ActionLoadingButton label={t("actionButton.stopping")} />
    ),
    [ButtonTypesEnum.restart]: <RestartButton handleAction={handleRestart} />,
    [ButtonTypesEnum.starting]: (
      <ActionLoadingButton label={t("actionButton.starting")} />
    ),
    [ButtonTypesEnum.delete]: <DeleteButton handleAction={handleDelete} />,
    [ButtonTypesEnum.deleting]: (
      <ActionLoadingButton label={t("actionButton.deleting")} />
    ),
    [ButtonTypesEnum.canceling]: (
      <DisabledButton label={t("disabledButton.canceling")} />
    ),
    [ButtonTypesEnum.deleted]: (
      <DisabledButton label={t("disabledButton.deleted")} />
    ),
    [ButtonTypesEnum.pending]: (
      <ActionLoadingButton label={t("disabledButton.pending")} />
    ),
  }

  // memoize so this isn't recalculated every time we fetch the workspace
  const [primaryAction, ...secondaryActions] = useMemo(
    () =>
      isUpdating
        ? [ButtonTypesEnum.updating, ...actions]
        : canBeUpdated
        ? [ButtonTypesEnum.update, ...actions]
        : actions,
    [actions, canBeUpdated, isUpdating],
  )

  return (
    <DropdownButton
      primaryAction={buttonMapping[primaryAction]}
      canCancel={canCancel}
      handleCancel={handleCancel}
      secondaryActions={secondaryActions.map((action) => ({
        action,
        button: buttonMapping[action],
      }))}
    />
  )
}
