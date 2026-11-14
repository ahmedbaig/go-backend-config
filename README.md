# go-backend-config

Typed configuration loading for Go services. Part of the `go-backend-*` family.

Each service reads one JSON document per config name (`my-app`, `my-app-logger`, `my-app-db`, ...). Lookup order:

1. Environment variable `<NAME>_SECRET` containing the JSON (`my-app-logger` → `MY_APP_LOGGER_SECRET`)
2. File `$CONFIG_DIR/<name>.json` (`CONFIG_DIR` defaults to `/app/var/secrets`)
3. The defaults you pass in

The JSON is decoded over the defaults, so a file only needs the keys it changes. The result is then checked with `validate:` struct tags ([go-playground/validator](https://github.com/go-playground/validator)) and an optional `Validate() error` method. Config values are never logged or included in errors.

## Install

```sh
go get github.com/ahmedbaig/go-backend-config
```

## Usage

```go
type AppConfig struct {
	Port        int             `json:"port" validate:"min=1,max=65535"`
	Host        string          `json:"host" validate:"required"`
	Environment string          `json:"environment" validate:"oneof=localhost testing staging production"`
	Timeout     config.Duration `json:"timeout"`
}

cfg, err := config.Load("my-app", AppConfig{
	Port:        8080,
	Host:        "0.0.0.0",
	Environment: "production",
	Timeout:     config.Duration(30 * time.Second),
})
```

`config.Duration` accepts `"1500ms"` / `"2m"` strings or a number of milliseconds.

### Options

| Option | Effect |
|---|---|
| `WithDir(dir)` | read files from `dir` instead of `$CONFIG_DIR` |
| `WithEnvKey(key)` | use a different environment variable name |
| `WithStrict()` | reject unknown JSON fields |
| `WithValidator(v)` | use your own `*validator.Validate` |
| `WithLookup(fn)` | replace `os.LookupEnv` (tests) |

`LoadWithSource` also returns where the value came from (`env`, `file` or `default`) and the path or variable name, so a service can log that without logging the contents. `MustLoad` panics on error, for use at startup.

## License

MIT
