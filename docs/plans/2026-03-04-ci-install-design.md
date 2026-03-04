# CI + Installation Design

## Repo
- Owner: `Alexy-vda/TokenEater-linux`
- Visibility: public
- Description: "Monitor your Claude AI token usage from the GNOME Shell status bar"

## CI — GitHub Actions

### `ci.yml` (push + PR)
- `go test ./...` and `go vet ./...` in `daemon/`
- Validates compilation

### `release.yml` (tag `v*`)
1. Cross-compile daemon: `linux/amd64` + `linux/arm64`
2. Package extras: GNOME extension + systemd service + D-Bus activation → `tokeneater-linux-extras.tar.gz`
3. Create GitHub Release with 3 assets:
   - `tokeneater-daemon-linux-amd64`
   - `tokeneater-daemon-linux-arm64`
   - `tokeneater-linux-extras.tar.gz`

## Installation
- One-liner: `bash <(curl -fsSL https://raw.githubusercontent.com/Alexy-vda/TokenEater-linux/main/install.sh)`
- Update `install.sh` URLs from `AThevon/TokenEater` → `Alexy-vda/TokenEater-linux`
- Update `README.md` URLs accordingly
- Remove `linux/` path prefixes (files are now at repo root)

## Release workflow
`git tag v1.0.0 && git push --tags` → CI builds + creates release automatically
