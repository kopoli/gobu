# Gobu

A traitful `go build` wrapper.

## Installation

```
$ go get github.com/kopoli/gobu
```

## Description

There are some hard-to-remember options to the `build` and `link` tools.  With
`gobu` one can list the wanted traits as simple words in the command line and
the appropriate commands will be generated.

## Supported traits

The following traits are supported:

- **debug**: Set `-x` build flag.
- **install**: Run `go install` instead of `go build`.
- **linux**: Set `GOOS=linux` environment variable.
- **nocgo**: Set `CGO_ENABLED=0` environment variable.
- **package**: After building creates a zip-package of the binary, README* and
  LICENSE files. Extra files can be added with the `GOBU_EXTRA_DIST`
  environment variable.
- **rebuild**: Set `-a` build flag.
- **shrink**: Set `-s -w` link flags.
- **static**: Set `-extldflags "-static"` link flags.
- **verbose**: Set `-v` build flag.
- **version**: Set the following go variables to the `main` package:

  * `main.timestamp`: Value of `time.Now().Format(time.RFC3339)`.
  * `main.version`: Output of `git describe --always --tags --dirty`.
  * `main.buildGOOS`: Value of `runtime.GOOS`.
  * `main.buildGOARCH`: Value of `runtime.GOARCH`.

- **windows**: Set `GOOS=windows` environment variable.

The following composite traits are supported:

- **default**: Sets the **version** trait. This is used if `gobu` is run
  without arguments.
- **release**: Sets the traits: **shrink**, **version**, **static** and
  **rebuild**.

If there are conflicting options (e.g. **linux** and **windows**) then the
latter will be in effect.

## Example

```
$ gobu shrink static nocgo
```

This will add the `-s -w -extldflags "-static"` flags to the linker, and set
the `CGO_ENABLED=0` environment variable

The binary packages of `gobu` are generated with the following command:

```
$ gobu release package
```

## License

MIT license