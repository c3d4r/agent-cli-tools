# agent-cli-tools

CLI tools for AI coding agents. Zero external dependencies — built entirely on Go's stdlib.

## Tools

### `e` — non-visual file editor

A line-and-content-addressed editor designed for terminals and agents. No interactive UI — every edit is a single command.

| Command | Description | Example |
|---------|-------------|---------|
| `set` | Replace a single line | `e set main.go 42 "    return nil"` |
| `setrange` | Replace a range of lines | `e setrange main.go 10-15 "new content"` |
| `delete` | Delete line(s) | `e delete main.go 10-15` |
| `insert` | Insert before line | `e insert main.go 1 "// Header"` |
| `append` | Insert after line | `e append main.go 5 "new line"` |
| `replace` | Exact string replace | `e replace main.go 'foo' 'bar'` |
| `after` | Insert after matching line | `e after main.go 'import (' '    "fmt"'` |
| `before` | Insert before matching line | `e before main.go 'func main' '// Entry point'` |
| `show` | Show file with line numbers | `e show main.go 40-50` |

**Flags:** `--all` (all occurrences), `--regex`, `--dry-run`, `--diff`, `--stdin`

### `lsp-cli` — LSP client for code intelligence

One-shot CLI commands that tap into language server intelligence. Auto-detects the language server from file extensions.

| Command | Description | Example |
|---------|-------------|---------|
| `definition` | Find where a symbol is defined | `lsp-cli def main.go:42:15` |
| `references` | Find all references to a symbol | `lsp-cli refs main.go:6:6` |
| `hover` | Show type signature and docs | `lsp-cli hover main.go:42:15` |
| `symbols` | List all symbols in a file | `lsp-cli syms main.go` |
| `diagnostics` | Show errors and warnings | `lsp-cli diag main.go` |
| `implementations` | Find interface implementations | `lsp-cli impl main.go:12:6` |
| `workspace-symbols` | Search symbols across project | `lsp-cli wsyms "Handler"` |

**Flags:** `-json`, `-server "cmd"`, `-root "dir"`, `-v`, `-timeout N`

**Location format:** `file:line:col` (1-indexed, matching compiler output)

## Install

### Install script (recommended)

```bash
curl -sL https://raw.githubusercontent.com/c3d4r/agent-cli-tools/main/install.sh | sh
```

Installs both tools to `~/.local/bin`. Override with `INSTALL_DIR`:

```bash
curl -sL https://raw.githubusercontent.com/c3d4r/agent-cli-tools/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

Install a specific version:

```bash
curl -sL https://raw.githubusercontent.com/c3d4r/agent-cli-tools/main/install.sh | VERSION=v0.1.0 sh
```

### Go install

```bash
go install github.com/c3d4r/agent-cli-tools/cmd/e@latest
go install github.com/c3d4r/agent-cli-tools/cmd/lsp-cli@latest
```

### Build from source

```bash
git clone https://github.com/c3d4r/agent-cli-tools
cd agent-cli-tools
go build -o e ./cmd/e/
go build -o lsp-cli ./cmd/lsp-cli/
```

## Supported language servers

`lsp-cli` auto-detects and launches the appropriate server. The server binary must be on your PATH.

| Language | Server | Install |
|----------|--------|---------|
| Go | `gopls` | `go install golang.org/x/tools/gopls@latest` |
| Python | `pylsp` / `pyright` | `pip install python-lsp-server` / `npm i -g pyright` |
| TypeScript/JS | `typescript-language-server` | `npm i -g typescript-language-server typescript` |
| Rust | `rust-analyzer` | Ships with rustup |
| C/C++ | `clangd` | Ships with LLVM |
| Java | `jdtls` | Eclipse JDT LS |
| Ruby | `solargraph` | `gem install solargraph` |

Use `-server "command args"` to override auto-detection.

## Architecture

```
cmd/e/main.go              e editor — all commands in one file
cmd/lsp-cli/main.go        lsp-cli entry point, subcommands, flag parsing
internal/lsp/client.go     LSP client: lifecycle, didOpen, request methods
internal/lsp/jsonrpc.go    JSON-RPC 2.0 transport with Content-Length framing
internal/lsp/types.go      LSP protocol types (subset needed for CLI)
internal/output/format.go  Output formatting (text and JSON)
internal/config/servers.go Language server detection and configuration
```

Zero external dependencies. ~2500 lines of Go.
