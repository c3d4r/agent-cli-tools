# CLAUDE.md — Agent instructions for this repo

## Build

```bash
go build ./cmd/e/
go build ./cmd/lsp-cli/
go vet ./...
```

No external dependencies. No test suite yet.

## Project structure

```
cmd/e/main.go              e editor (single file, ~740 lines)
cmd/lsp-cli/main.go        lsp-cli CLI entry point (~520 lines)
internal/lsp/client.go     LSP client lifecycle + request methods
internal/lsp/jsonrpc.go    JSON-RPC 2.0 over stdio
internal/lsp/types.go      LSP protocol types
internal/output/format.go  Output formatter (text + JSON)
internal/config/servers.go Language server auto-detection
docs/lsp-cli-design.md     Original design document
```

## Tool reference

### `e` — non-visual file editor

Line-addressed commands (1-indexed):

```bash
e set <file> <line> <text>            # Replace a single line
e setrange <file> <from>-<to> <text>  # Replace a range of lines
e delete <file> <line|from-to>        # Delete line(s)
e insert <file> <line> <text>         # Insert text before line
e append <file> <line> <text>         # Insert text after line
```

Content-addressed commands:

```bash
e replace <file> <old> <new>          # Exact string replace (first match)
e after <file> <match> <text>         # Insert text after matching line
e before <file> <match> <text>        # Insert text before matching line
```

Display:

```bash
e show <file> [from-to]               # Show file with line numbers
```

Flags: `--all` (all matches), `--regex`, `--dry-run`, `--diff`, `--stdin`

Examples:

```bash
e set main.go 42 "    return nil"
e delete main.go 10-15
e replace main.go 'func Foo()' 'func Bar(ctx context.Context)'
e after main.go 'import (' '    "context"'
e --diff replace main.go 'oldFunc' 'newFunc'
echo -e "line1\nline2" | e --stdin insert main.go 5
```

### `lsp-cli` — LSP client CLI

```bash
lsp-cli definition <file:line:col>     # Find definition of symbol
lsp-cli references <file:line:col>     # Find all references to symbol
lsp-cli hover <file:line:col>          # Show type/docs for symbol
lsp-cli symbols <file>                 # List symbols in file
lsp-cli diagnostics <file> [file...]   # Show diagnostics (errors/warnings)
lsp-cli implementations <file:line:col> # Find implementations of interface
lsp-cli workspace-symbols <query>      # Search symbols across workspace
```

Short aliases: `def`, `refs`, `syms`, `diag`, `impl`, `wsyms`

Flags: `-json`, `-server "cmd"`, `-root "dir"`, `-v`, `-timeout N`

Location format: `file:line:col` (1-indexed). Requires a language server on PATH (e.g. `gopls` for Go).

Examples:

```bash
lsp-cli definition ./server/handler.go:42:15
lsp-cli references ./pkg/auth/token.go:28:6
lsp-cli --json hover ./server/handler.go:42:15
lsp-cli diagnostics ./server/handler.go ./server/middleware.go
lsp-cli -server "pyright-langserver --stdio" definition app.py:10:5
```

## When to use each tool

- **`e`**: Edit files by line number or content match. Use when you know the exact line or text to change. Ideal for scripted/automated edits.
- **`lsp-cli`**: Navigate and understand code semantically. Use to find definitions, references, implementations, and diagnostics. Supplements grep/ripgrep with compiler-level accuracy.

## Conventions

- Zero external dependencies — stdlib only
- Both tools share one Go module and one version tag
- Module path: `github.com/c3d4r/agent-cli-tools`
- Cross-compiled for linux/darwin, amd64/arm64
- Static binaries with CGO_ENABLED=0
