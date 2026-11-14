# Releasing Orbix

Releases are built by GitHub Actions from version tags. A release tag is accepted only when its commit is on `main` and the verification job passes.

## Before merging to main

1. Confirm CI passes on Linux x64, macOS Intel, macOS Apple Silicon, and Windows x64.
2. Run `make ci` with Go 1.27.3 or newer.
3. Update `CHANGELOG.md` with user-visible changes.
4. Verify `orbix doctor`, the TUI, and at least one public CLI command locally.
5. Confirm no credentials, local databases, logs, or diagnostics archives are staged.

## Publish

Use a Semantic Versioning tag:

```sh
git switch main
git pull --ff-only origin main
git tag -s v1.0.0 -m "Orbix v1.0.0"
git push origin v1.0.0
```

If signed tags are not available in the maintainer environment, use an annotated tag with `git tag -a`. Never move or reuse a published version tag; publish a new patch version instead.

The release workflow builds and packages:

- Windows x64 and Arm64 ZIP archives
- macOS Intel and Apple Silicon tarballs
- Linux x64 and Arm64 tarballs
- `SHA256SUMS`
- GitHub build-provenance attestations

Build metadata is injected into `orbix version`: tag, full commit SHA, and UTC build time.

## Verify the published release

1. Confirm all six archives and `SHA256SUMS` are attached.
2. Download at least one archive and verify its checksum.
3. Run `gh attestation verify ARCHIVE --repo talha-shah2031/orbix`.
4. Extract the archive and run `orbix version` and `orbix doctor`.
5. Confirm the generated release notes match `CHANGELOG.md`.

Windows Authenticode signing and Apple notarization are not configured. Checksums and GitHub provenance establish build integrity, but those operating systems may still show first-run reputation warnings.
