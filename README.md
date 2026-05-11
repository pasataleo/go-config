# go-template

A template repository for Go projects. Requires macOS or Linux.

## Setup

After creating a new repository from this template, run:

```sh
./setup.sh
```

This will:

- Update the Go module path to match your repository name
- Configure the project as an executable or library
- Install a git pre-commit hook that runs `make all`
- Set up the release workflow for the chosen project type
- Delete itself

## Make targets

- `make all` — run tidy, generate, fmt, lint, test, and build
- `make tidy` — run `go mod tidy`
- `make generate` — run `go generate ./...`
- `make fmt` — run `go fmt ./...`
- `make lint` — install and run golangci-lint
- `make test` — run `go test ./...`
- `make build` — build the binary (or verify compilation for libraries); local builds report version `dev`

## Releasing

Update `CHANGELOG.md` with the changes for the release, then tag the commit:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow will create a GitHub release using the contents of `CHANGELOG.md` and clear it ready for the next release.
