# Lobot

Kubernetes TUI client and visualizer.

## Development

Lobot requires Go 1.25 or newer.

```sh
go build ./...
go test ./...
```

## Local Kubernetes environment

With Docker running, create or reconcile the lightweight k3d test environment:

```sh
./scripts/setup-k3d.sh
```

See [`testdata/k3d/README.md`](testdata/k3d/README.md) for fixtures, overrides,
and cleanup instructions.
