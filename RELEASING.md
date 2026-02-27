# Releasing

## 1) Module path

Current module path is already configured in [go.mod](go.mod):

```go
module github.com/iSherlott/bullMQ
```

## 2) Run quality checks

```bash
go mod tidy
go test ./...
```

## 3) Prepare changelog

Move relevant items from `Unreleased` to a new version entry.

## 4) Create release tag

```bash
git tag v0.2.0
git push origin v0.2.0
```

After pushing the tag, GitHub Actions workflow [release.yml](.github/workflows/release.yml) will:
- run `go mod tidy`
- run `go test ./...`
- publish a GitHub Release automatically

## 5) Consumers install

```bash
go get github.com/iSherlott/bullMQ@v0.2.0
```
