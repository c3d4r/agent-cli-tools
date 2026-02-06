# lsp-cli

A command-line LSP client designed for AI coding agents.

**Zero external dependencies** — built entirely on Go's stdlib.

## Why

AI coding agents navigate codebases with grep and file reads. This works until
you need to distinguish a definition from a usage, follow an interface to its
implementations, or check for type errors without running a full build.

`lsp-cli` exposes language server intelligence as one-shot CLI commands:

```
lsp-cli definition main.go:44:9     → exact source location
lsp-cli references main.go:6:6      → all usages of a symbol
lsp-cli hover main.go:44:9          → type signature + docs
lsp-cli symbols main.go             → file structure tree
lsp-cli diagnostics main.go         → errors without building
lsp-cli implementations main.go:12:6 → interface → concrete types
```

## Install

```bash
go install github.com/c3d4r/cli-lsp-mcp/cmd/lsp-cli@latest
```

Or build from source:

```bash
git clone https://github.com/c3d4r/cli-lsp-mcp
cd cli-lsp-mcp
go build -o lsp-cli ./cmd/lsp-cli/
```

Requires a language server on PATH (e.g., `gopls` for Go).

## Commands

| Command | Description | Example |
|---------|-------------|---------|
| `definition` | Find where a symbol is defined | `lsp-cli def main.go:44:9` |
| `references` | Find all references to a symbol | `lsp-cli refs main.go:6:6` |
| `hover` | Show type signature and docs | `lsp-cli hover main.go:44:9` |
| `symbols` | List all symbols in a file | `lsp-cli syms main.go` |
| `diagnostics` | Show errors and warnings | `lsp-cli diag main.go` |
| `implementations` | Find interface implementations | `lsp-cli impl main.go:12:6` |
| `workspace-symbols` | Search symbols across project | `lsp-cli wsyms "Handler"` |

All commands have short aliases shown in the table.

## Flags

```
-json       Output as JSON
-server     Override language server command (e.g., -server "gopls serve")
-root       Set workspace root directory (default: auto-detected)
-v          Verbose output (show server notifications)
-timeout    Timeout in seconds (default: 30)
```

## Location Format

Locations use `file:line:col` format (1-indexed), matching compiler output and
grep results. The CLI converts to LSP's 0-indexed format internally.

## Supported Language Servers

Auto-detection works for:

| Language | Server |
|----------|--------|
| Go | gopls |
| Python | pylsp, pyright |
| TypeScript/JS | typescript-language-server |
| Rust | rust-analyzer |
| C/C++ | clangd |
| Java | jdtls |
| Ruby | solargraph |

Use `-server "command args"` to override for any other server.

## Architecture

```
cmd/lsp-cli/main.go        CLI entry point, subcommands, flag parsing
internal/lsp/types.go       LSP protocol types (subset needed for CLI)
internal/lsp/jsonrpc.go     JSON-RPC 2.0 transport with Content-Length framing
internal/lsp/client.go      LSP client: lifecycle, didOpen, all request methods
internal/output/format.go   Output formatting (text and JSON)
internal/config/servers.go  Language server detection and configuration
```

1875 lines of Go. Zero dependencies.
