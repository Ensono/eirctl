# Self Update

This package provides the ability to plug in another subcommand into your cobra app, that will expose the below subcommand.

```sh
Updates the binary to the specified or latest version.

Usage:
  eirctl update [flags]

Aliases:
  update, self-update

Flags:
      --baseUrl string   base url for the github release repository (default "https://github.com/Ensono/eirctl/releases")
  -h, --help             help for update
      --version string   specific version to update to. (default "latest")

Global Flags:
  -c, --config eirctl.yaml   config file to use - eirctl.yaml. For backwards compatibility it also accepts taskctl.yaml and tasks.yaml (default "eirctl.yaml")
  -d, --debug                enable debug level logging
      --dry-run              dry run
      --no-summary           show summary
  -q, --quiet                quite mode
      --verbose              enable trace level logging
```

The above example is from embedding in the [eirctl](https://github.com/Ensono/eirctl) utility

## Github Releases

It supports GitHub releases OOTB, but custom functions for GetVersion can be provided, you can see `ExampleUpdateCmd_withOwnGetFunc` for more details.
