# Releasing BNX

This guide prepares a GitHub release from a **standalone BNX repository**. The release workflow builds and attaches archives; it does not publish anything until a maintainer pushes a version tag.

## 1. Confirm the repository

Run these commands from the BNX repository root:

```sh
git rev-parse --show-toplevel
git remote -v
```

Confirm the top-level directory is the standalone BNX checkout and that `origin` is your BNX repository. Do not run the `git init` sequence from inside a different project or monorepo. If `origin` is absent, add it; if it points elsewhere, update it deliberately:

```sh
git remote add origin https://github.com/<OWNER>/<REPOSITORY>.git
# Or, when origin already exists:
git remote set-url origin https://github.com/<OWNER>/<REPOSITORY>.git
```

For this project, use the repository URL you own, for example `https://github.com/talha-shah2031/BNX.git`. Confirm with `git remote -v` before pushing. Never replace a remote blindly: this checkout may be nested under a different Git repository.

## 2. Prepare and verify

Review the changes and ensure the worktree contains only release-ready BNX changes. Then run the project checks:

```sh
go mod verify
go vet ./...
go test -race ./...
golangci-lint run ./...
govulncheck ./...
goreleaser check
```

The GoReleaser configuration sets `CGO_ENABLED=0` and builds Windows ZIP archives for amd64 and arm64. It creates checksums and archive SBOMs. No Binance credentials are needed to build or release. The GitHub Actions runner is Linux, but the release artifacts are Windows executables.

## 3. Push code and create a version tag

Commit the reviewed release changes using the project's Conventional Commit style. Push the branch and then create and push an annotated semantic version tag. Example for the first release:

```sh
git push -u origin main
git tag -a v0.1.0 -m "BNX v0.1.0"
git push origin v0.1.0
```

Pushing a tag matching `v*` starts `.github/workflows/release.yml`. GitHub Actions runs GoReleaser and creates a GitHub Release with Windows archives, `checksums.txt`, and SBOM files. The workflow needs the repository's standard `GITHUB_TOKEN` with contents-write permission; no personal access token is required.

## 4. Inspect the release

Wait for the **Release** GitHub Actions workflow to finish. Check the release page for both Windows architecture archives, checksums, and SBOMs. Download an archive and verify its checksum before installation. To correct a release, prepare a new patch version (for example, `v0.1.1`) rather than force-moving an already published tag.

## Local snapshot (does not publish)

To validate the archives locally without creating a GitHub Release, run:

```sh
goreleaser release --snapshot --clean
```

This writes local artifacts under `dist/`. Do not run `goreleaser release --clean` manually for a published release; the tag-triggered workflow performs that step.
