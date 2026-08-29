# GitHub Releases (GoReleaser)

Prebuilt binaries for **macOS**, **Linux**, and **Windows** are published automatically when a version tag is pushed.

## Install from a release

1. Open [GitHub Releases](https://github.com/shunvel/mcp-breaker/releases)
2. Download the archive for your platform, for example:
   - `mcp-breaker_0.2.0_darwin_arm64.tar.gz` — Apple Silicon Mac
   - `mcp-breaker_0.2.0_darwin_amd64.tar.gz` — Intel Mac
   - `mcp-breaker_0.2.0_linux_amd64.tar.gz` — Linux x86_64
   - `mcp-breaker_0.2.0_windows_amd64.zip` — Windows
3. Extract and move the binary onto your `PATH`:

```bash
# macOS / Linux example
tar xzf mcp-breaker_*_darwin_arm64.tar.gz
sudo mv mcp-breaker /usr/local/bin/
mcp-breaker help
```

```powershell
# Windows example (PowerShell)
Expand-Archive mcp-breaker_*_windows_amd64.zip -DestinationPath .
# Add the folder containing mcp-breaker.exe to your PATH
```

Verify checksums with `checksums.txt` on the release page.

## go install (Go 1.24+)

```bash
go install github.com/shunvel/mcp-breaker/cmd/mcp-breaker@latest
# or pin a version:
go install github.com/shunvel/mcp-breaker/cmd/mcp-breaker@v0.2.0
```

Ensure `$GOBIN` or `$GOPATH/bin` is on your `PATH`.

## Publish a new release (maintainers)

1. Merge changes to `main` and ensure CI is green
2. Tag and push:

```bash
git tag v0.2.1
git push origin v0.2.1
```

3. The [Release workflow](../../.github/workflows/release.yml) runs GoReleaser, which:
   - Builds cross-platform binaries
   - Creates/updates the GitHub Release with assets and `checksums.txt`
   - Commits an updated Homebrew formula to `Formula/mcp-breaker.rb` on `main`

## Dry-run locally

Install [GoReleaser](https://goreleaser.com/) and run:

```bash
goreleaser release --snapshot --clean
```

Artifacts land in `dist/` without publishing.

Validate config only:

```bash
goreleaser check
```
