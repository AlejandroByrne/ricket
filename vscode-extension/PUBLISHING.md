# Publishing the Ricket VS Code Extension

## Prerequisites

1. **Node.js** ≥ 18 and **npm**.
2. **vsce** — the VS Code Extension CLI:
   ```bash
   npm install -g @vscode/vsce
   ```
3. **Azure DevOps Personal Access Token (PAT)** — required for publishing.

## 1. Register a Publisher

If you don't already have a publisher on the VS Code Marketplace:

1. Go to https://marketplace.visualstudio.com/manage
2. Sign in with a Microsoft account.
3. Click **Create Publisher**, choose an ID (e.g., `AlejandroByrne`) and display name.
4. The `publisher` field in `package.json` must match this publisher ID exactly.

## 2. Build Cross-Platform Go Binaries

From the repo root, build ricket for each target platform:

```bash
# Linux
GOOS=linux   GOARCH=amd64 go build -o vscode-extension/bin/ricket-linux-x64       ./cmd/ricket
GOOS=linux   GOARCH=arm64 go build -o vscode-extension/bin/ricket-linux-arm64      ./cmd/ricket

# macOS
GOOS=darwin  GOARCH=amd64 go build -o vscode-extension/bin/ricket-darwin-x64       ./cmd/ricket
GOOS=darwin  GOARCH=arm64 go build -o vscode-extension/bin/ricket-darwin-arm64      ./cmd/ricket

# Windows
GOOS=windows GOARCH=amd64 go build -o vscode-extension/bin/ricket-win32-x64.exe    ./cmd/ricket
GOOS=windows GOARCH=arm64 go build -o vscode-extension/bin/ricket-win32-arm64.exe   ./cmd/ricket
```

On **Windows PowerShell** cross-compilation uses environment variables:

```powershell
$env:GOOS="linux";   $env:GOARCH="amd64"; & 'C:\Program Files\Go\bin\go.exe' build -o vscode-extension/bin/ricket-linux-x64   ./cmd/ricket
$env:GOOS="darwin";  $env:GOARCH="arm64"; & 'C:\Program Files\Go\bin\go.exe' build -o vscode-extension/bin/ricket-darwin-arm64 ./cmd/ricket
# ... etc. Reset with: Remove-Item Env:\GOOS; Remove-Item Env:\GOARCH
```

## 3. Package the Extension

```bash
cd vscode-extension
vsce package
# → ricket-0.3.0.vsix
```

This produces a `.vsix` file that can be installed locally (`code --install-extension ricket-0.3.0.vsix`) or published to the Marketplace.

## 4. Publish to the Marketplace

```bash
# First time: log in with your PAT
vsce login AlejandroByrne

# Publish
cd vscode-extension
vsce publish
```

Or publish in one step with the PAT inline:

```bash
vsce publish -p <YOUR_PAT>
```

### Bumping Versions

```bash
vsce publish minor   # 0.3.0 → 0.4.0
vsce publish patch   # 0.3.0 → 0.3.1
```

Keep `package.json` version in sync with the Go binary's `--version` output.

## 5. CI/CD Pipeline (GitHub Actions)

Here's a reference workflow that builds Go binaries for all platforms and publishes the extension on tag push:

```yaml
name: Release Extension

on:
  push:
    tags: ["v*"]

jobs:
  build-and-publish:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux   goarch: amd64  binary: ricket-linux-x64
          - goos: linux   goarch: arm64  binary: ricket-linux-arm64
          - goos: darwin  goarch: amd64  binary: ricket-darwin-x64
          - goos: darwin  goarch: arm64  binary: ricket-darwin-arm64
          - goos: windows goarch: amd64  binary: ricket-win32-x64.exe
          - goos: windows goarch: arm64  binary: ricket-win32-arm64.exe
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - name: Build Go binary
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: go build -o vscode-extension/bin/${{ matrix.binary }} ./cmd/ricket
      - uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.binary }}
          path: vscode-extension/bin/${{ matrix.binary }}

  publish:
    needs: build-and-publish
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          path: vscode-extension/bin
          merge-multiple: true
      - uses: actions/setup-node@v4
        with:
          node-version: 20
      - name: Install deps & publish
        working-directory: vscode-extension
        env:
          VSCE_PAT: ${{ secrets.VSCE_PAT }}
        run: |
          chmod +x bin/*
          npm ci
          npm install -g @vscode/vsce
          vsce publish -p "$VSCE_PAT"
```

**Secret required:** Add `VSCE_PAT` to your GitHub repo secrets (Settings → Secrets → Actions).

## Platform-Specific Extensions (Alternative)

For smaller download sizes, you can publish **platform-specific** extensions using `vsce`'s `--target` flag:

```bash
vsce package --target linux-x64
vsce package --target darwin-arm64
vsce package --target win32-x64
# ... etc.
```

This requires separate packaging per platform but means each user only downloads their platform's binary. See the [vsce docs on platform-specific extensions](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#platformspecific-extensions) for details.
