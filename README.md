# m

`m` is a tiny starter CLI app written in Go.

## Prerequisites

- Go 1.25+

## Run locally

```bash
go run ./cmd/m
go run ./cmd/m help
go run ./cmd/m version
```

## Build a binary

```bash
make build
./bin/m help
```

## Install globally

```bash
make install
m help
```

## Project layout

- `cmd/m/main.go` - CLI entrypoint
- `go.mod` - Go module metadata
