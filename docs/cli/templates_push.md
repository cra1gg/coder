<!-- DO NOT EDIT | GENERATED CONTENT -->

# templates push

Push a new template version from the current directory or as specified by flag

## Usage

```console
coder templates push [template]
```

## Options

### --parameter-file

Specify a file path with parameter values.

### --variables-file

Specify a file path with values for Terraform-managed variables.

### --variable

Specify a set of values for Terraform-managed variables.

### --provisioner-tag, -t

Specify a set of tags to target provisioner daemons.

### --name

Specify a name for the new template version. It will be automatically generated if not provided.

### --always-prompt

Always prompt all parameters. Does not pull parameter values from active template version.

### --yes, -y

Bypass prompts.

### --directory, -d

|         |                |
| ------- | -------------- |
| Default | <code>.</code> |

Specify the directory to create from, use '-' to read tar from stdin.