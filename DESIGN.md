# cli-lsp-mcp: An LSP Client CLI for AI Coding Agents

## 1. Why an Agent Needs LSP (Not Just Grep)

### The Problem: Agents Navigate Code Like Tourists With a Phrasebook

Today, AI coding agents (Claude Code, Cursor, Aider, etc.) understand codebases
through text-based tools: `grep`, `ripgrep`, `find`, `glob`, and reading files.
This is like navigating a foreign city by searching for street signs that contain
a particular word. It works surprisingly well — until it doesn't.

**What grep/ripgrep gives an agent:**
- Find files matching a name pattern
- Find lines matching a text pattern
- Read file contents

**What grep/ripgrep *cannot* do:**
- Distinguish a definition from a usage from a shadowed name from a string literal
- Follow an interface to its implementations
- Resolve an import to the actual source file (especially across modules/packages)
- Determine the type of a variable or expression
- Find all callers of a function (not just text matches of its name)
- Understand scope — a local `ctx` vs a package-level `ctx` vs a field named `ctx`
- Navigate through generics, type aliases, embedded structs, trait impls
- Detect errors and diagnostics *before* running the compiler

### Concrete Scenarios Where LSP Beats Grep

| Task | Grep Approach | LSP Approach |
|------|--------------|-------------|
| "Find where `HandleRequest` is defined" | `rg "func HandleRequest"` — works if it's a top-level func, fails for methods, interface decls, or if it's defined in a dependency | `textDocument/definition` on any usage — always correct, works across packages |
| "Find all callers of `Validate()`" | `rg "Validate\("` — returns every `Validate` call on *any* type, string matches in comments, test assertions | `textDocument/references` — returns only actual call sites of *that specific* `Validate` |
| "What type does this function return?" | Read the source, parse mentally, hope it's not inferred | `textDocument/hover` — returns the full resolved type signature |
| "List all functions in this file" | `rg "^func "` — fragile, misses methods, lambda-heavy langs | `textDocument/documentSymbol` — structured, complete, hierarchical |
| "Find all types that implement `io.Reader`" | Nearly impossible with grep | `textDocument/implementation` — direct answer |
| "Are there any errors in my changes?" | Run the full build and parse output | `textDocument/diagnostic` — incremental, fast, structured |
| "What can I call on this object?" | Read the type def, its embedded types, its methods... | `textDocument/completion` — full list with signatures |
| "Rename `userID` to `userAccountID` safely" | `sed` or find-replace — breaks if there's a `userIDToken` or `getUserID` | `textDocument/rename` — semantically correct across the project |

### The Key Insight

An LSP server has already done the hard work of parsing, type-checking, and
building a semantic model of the code. When an agent uses grep, it's throwing
away all that understanding and trying to reconstruct it from raw text. This is
both **slower** (multiple rounds of grep + read + grep to resolve ambiguity) and
**less reliable** (text matching is inherently fuzzy).

An LSP client CLI lets the agent tap into the compiler's understanding directly.

---

## 2. How an Agent Would Use an LSP Client CLI

### Usage Model: One-Shot Commands, Not a REPL

The key design insight from gopls is that an LSP client CLI should support
**one-shot invocations** that handle the full LSP lifecycle internally:

```
# Agent wants to understand a symbol
$ lsp-cli definition ./server/handler.go:42:15
/home/user/project/pkg/auth/token.go:28:6

# Agent wants to find all references
$ lsp-cli references ./server/handler.go:42:15
/home/user/project/server/handler.go:42:15
/home/user/project/server/middleware.go:18:3
/home/user/project/server/handler_test.go:55:8

# Agent wants to see diagnostics after editing
$ lsp-cli diagnostics ./server/handler.go
./server/handler.go:42:15: error: cannot use string as int
./server/handler.go:58:2: warning: unused variable 'tmp'

# Agent wants to understand a symbol's type
$ lsp-cli hover ./server/handler.go:42:15
func ValidateToken(token string) (*Claims, error)
ValidateToken checks the JWT token and returns the parsed claims.

# Agent wants to see the structure of a file
$ lsp-cli symbols ./server/handler.go
func HandleRequest (line 15)
  var ctx (line 16)
  var req (line 22)
func HandleError (line 45)
type Server struct (line 60)
  field logger (line 61)
  field db (line 62)
  method Start (line 65)
  method Stop (line 80)

# Agent wants to find implementations of an interface
$ lsp-cli implementations ./pkg/store/store.go:10:6
/home/user/project/pkg/store/postgres.go:15:6
/home/user/project/pkg/store/memory.go:8:6

# Agent wants to do a workspace-wide symbol search
$ lsp-cli workspace-symbols "Handler"
server/handler.go:15:6 func HandleRequest
server/handler.go:45:6 func HandleError
server/handler.go:60:6 type Server
pkg/grpc/handler.go:12:6 type GRPCHandler

# Agent wants a safe rename
$ lsp-cli rename ./server/handler.go:42:15 "ValidateJWTToken"
# outputs a diff or list of edits the agent can review/apply
```

### The Agent Workflow

A typical agent session currently looks like:

```
1. grep for symbol name           → ambiguous results
2. read 3 candidate files         → find the right one
3. grep for usages                → noisy results
4. read more files to disambiguate → context window filling up
5. make the change
6. run the build to check         → slow feedback loop
7. fix errors, repeat
```

With an LSP CLI, this becomes:

```
1. lsp-cli definition file:line:col  → exact location
2. lsp-cli references file:line:col  → exact call sites
3. make the change
4. lsp-cli diagnostics file          → instant error check
5. done (or fix and repeat from 4)
```

**Fewer tool calls, less context consumed, higher accuracy.**

### Output Format Considerations

For agent consumption, structured output is critical. The CLI should support:

- **Default (human-readable)**: `file.go:line:col: description` — grep-compatible
- **JSON (`--json`)**: Machine-parseable, includes all metadata
- **Quiet (`-q`)**: Just locations, no descriptions — for piping

The grep-compatible default is important: agents already know how to parse
`file:line:col` format from compiler output and ripgrep results.

---

## 3. Research: Libraries and Approaches

### gopls CLI — The Reference Implementation

gopls (the Go language server) has a well-designed CLI mode that serves as the
primary reference for this project.

**Architecture:**
- `gopls` can run as a long-lived daemon (LSP over JSON-RPC on stdio/socket)
- CLI subcommands internally start a server instance, send the LSP request, and
  return the result — effectively a one-shot lifecycle
- Subcommands: `definition`, `references`, `hover`, `symbols`, `implementation`,
  `rename`, `signature`, `highlight`, `folding_ranges`, `format`, `imports`,
  `workspace_symbol`, `check` (diagnostics)
- Location format: `file.go:line:col` (1-indexed, as LSP uses 0-indexed internally)
- Supports `-json` flag for structured output

**Key lessons from gopls CLI:**
1. One-shot commands hide LSP lifecycle complexity
2. File:line:col is the natural addressing scheme
3. JSON output mode is essential for programmatic use
4. Workspace root is auto-detected (go.mod, .git, etc.)
5. The daemon mode for speed is a nice optimization but not essential for MVP

### Go Libraries

| Library | Description | Notes |
|---------|-------------|-------|
| `go.lsp.dev/protocol` | Complete LSP type definitions for Go | Well-maintained, auto-generated from the LSP spec. This is the types package. |
| `go.lsp.dev/jsonrpc2` | JSON-RPC 2.0 implementation | Used by many Go LSP implementations. Handles the transport layer. |
| `golang.org/x/tools/gopls` | gopls source itself | Reference for how to build CLI on top of LSP. Internal packages not importable. |
| `go.lsp.dev/pkg` | Utility package for LSP URI handling etc. | Small helpers. |

**Recommended Go stack:**
- `go.lsp.dev/protocol` for LSP types
- `go.lsp.dev/jsonrpc2` for JSON-RPC transport
- `cobra` or just `flag`/subcommands for CLI structure

### Python Libraries

| Library | Description | Notes |
|---------|-------------|-------|
| `pygls` | Python Generic Language Server framework | Primarily for *building* LSP servers, but has client capabilities too. |
| `lsprotocol` | Python LSP type definitions | Auto-generated from the spec, used by pygls. Complete type coverage. |
| `sansio-lsp-client` | Sans-IO LSP client library | Designed for building clients. Sans-IO means you control the transport. |
| `multilspy` (Microsoft) | Multi-language LSP client | From Microsoft Research. Designed specifically for using LSP from code (not an editor). Supports Python, Java, Rust, C#, Go out of the box. |

**Recommended Python stack:**
- `lsprotocol` for types
- `pygls` client capabilities or `sansio-lsp-client` for the client
- `click` or `typer` for CLI structure

### Existing Similar Projects

1. **gopls CLI itself** — The gold standard, but Go-only.
2. **`multilspy`** (Microsoft Research) — A Python library that wraps multiple
   LSP servers. Designed for programmatic use rather than CLI, but very close to
   what we want. Handles server lifecycle, workspace setup, etc.
3. **MCP LSP servers** — There are emerging MCP servers that expose LSP
   capabilities as MCP tools. These are the natural evolution of this idea but
   couple to the MCP protocol.
4. **`lsp-ws-proxy`** — WebSocket proxy for LSP servers, for browser-based
   editors. Different use case but interesting architecture.

### Language Server Availability

For the CLI to be useful, the target language needs a good LSP server. The good
news is that all major languages have mature servers:

| Language | Server | Install |
|----------|--------|---------|
| Go | `gopls` | `go install golang.org/x/tools/gopls@latest` |
| Python | `pylsp` / `pyright` | `pip install python-lsp-server` / `npm install pyright` |
| TypeScript/JS | `typescript-language-server` | `npm install -g typescript-language-server` |
| Rust | `rust-analyzer` | Ships with rustup |
| C/C++ | `clangd` | Ships with LLVM |
| Java | `jdtls` | Eclipse JDT LS |

---

## 4. MVP Implementation Plan

### Recommendation: Go

**Why Go over Python for this project:**
1. Single binary distribution — no runtime dependency, no virtualenv
2. The primary LSP types library (`go.lsp.dev/protocol`) is excellent
3. gopls source code serves as a reference written in the same language
4. Fast startup time matters for one-shot CLI invocations
5. Easy cross-compilation for different platforms
6. This project is itself a Go-workflow tool

### Architecture

```
lsp-cli
├── main.go                    # Entry point, subcommand dispatch
├── cmd/
│   ├── root.go                # Root command, global flags
│   ├── definition.go          # textDocument/definition
│   ├── references.go          # textDocument/references
│   ├── hover.go               # textDocument/hover
│   ├── symbols.go             # textDocument/documentSymbol
│   ├── diagnostics.go         # textDocument/diagnostic (+ publishDiagnostics)
│   ├── implementations.go     # textDocument/implementation
│   ├── workspace_symbols.go   # workspace/symbol
│   └── rename.go              # textDocument/rename (preview mode)
├── lsp/
│   ├── client.go              # LSP client: lifecycle, initialize, shutdown
│   ├── transport.go           # JSON-RPC over stdio (spawn & communicate with server)
│   ├── document.go            # textDocument/didOpen management
│   └── uri.go                 # file:// URI <-> filesystem path conversion
├── output/
│   ├── format.go              # Output formatting interface
│   ├── text.go                # Human-readable (file:line:col) formatter
│   └── json.go                # JSON formatter
├── config/
│   └── servers.go             # Language server registry & detection
└── go.mod
```

### Phase 1: Foundation (MVP Core)

**Goal:** Get a working `lsp-cli definition` and `lsp-cli hover` command against gopls.

**Steps:**

1. **Project scaffolding**
   - Initialize Go module
   - Set up `cobra` CLI structure with root command
   - Global flags: `--json`, `--server <cmd>`, `--root <dir>`, `--verbose`

2. **LSP transport layer** (`lsp/transport.go`)
   - Spawn a language server as a subprocess (e.g., `gopls serve`)
   - Communicate over stdio using JSON-RPC 2.0
   - Handle Content-Length framing (LSP base protocol)
   - Use `go.lsp.dev/jsonrpc2` for the JSON-RPC layer

3. **LSP client lifecycle** (`lsp/client.go`)
   - Send `initialize` with project root and client capabilities
   - Handle `initialized` notification
   - Send `textDocument/didOpen` to register files with the server
   - Send `shutdown` + `exit` on completion
   - Timeout handling (servers can be slow to initialize)

4. **Location parsing**
   - Parse `file.go:line:col` arguments into LSP `TextDocumentPositionParams`
   - Convert between 1-indexed (human) and 0-indexed (LSP) positions
   - Handle relative and absolute paths, convert to `file://` URIs

5. **`definition` command** (`cmd/definition.go`)
   - Open file → send `textDocument/definition` → print result locations
   - Handle `Location` and `LocationLink` response variants

6. **`hover` command** (`cmd/hover.go`)
   - Open file → send `textDocument/hover` → print markdown content
   - Strip markdown fences for terminal display in text mode

7. **Output formatting**
   - Text mode: `filepath:line:col: content`
   - JSON mode: Full LSP response as JSON

### Phase 2: Navigation Commands

**Goal:** Full suite of navigation commands that make an agent self-sufficient.

8. **`references` command** — `textDocument/references`
   - Include/exclude declarations via flag
   - Output as location list

9. **`symbols` command** — `textDocument/documentSymbol`
   - Tree display showing symbol hierarchy (functions, types, fields, methods)
   - Flat display option for piping

10. **`workspace-symbols` command** — `workspace/symbol`
    - Query-based search across the entire workspace
    - Useful for "find all types/functions matching a pattern"

11. **`implementations` command** — `textDocument/implementation`
    - Find concrete implementations of interfaces/abstract types

### Phase 3: Diagnostics and Editing Support

**Goal:** Fast feedback loop for the agent — check errors without running the build.

12. **`diagnostics` command** — `textDocument/diagnostic` + `publishDiagnostics`
    - Open file(s), wait for diagnostics, report them
    - Severity filtering (errors only, warnings+errors, all)
    - This is tricky: diagnostics are *pushed* by the server, not requested
    - Implementation: open file, wait N seconds or until diagnostics arrive
    - Alternative: use `textDocument/didChange` to notify of edits, then collect

13. **`rename` command** — `textDocument/rename` (dry-run/preview)
    - Show the edit that *would* be applied as a diff
    - Optionally apply the edits directly
    - Agent can review the diff before applying

### Phase 4: Server Management and Multi-Language

14. **Server auto-detection** (`config/servers.go`)
    - Detect language from file extension
    - Look up known server commands (gopls, pyright, rust-analyzer, etc.)
    - Check if the server binary is available on PATH
    - Allow override via `--server` flag or config file

15. **Daemon mode** (performance optimization)
    - Keep the server running between invocations via a socket
    - `lsp-cli daemon start` / `lsp-cli daemon stop`
    - One-shot commands connect to daemon if running, otherwise spawn fresh
    - This is the gopls model and dramatically improves latency

16. **Config file**
    - `.lsp-cli.yaml` in project root or `~/.config/lsp-cli/config.yaml`
    - Server command overrides per language
    - Default output format preferences

### Phase 5: MCP Bridge (Future)

17. **MCP server mode** — Expose the LSP commands as MCP tools
    - `lsp-cli mcp-serve` — runs as an MCP server
    - Each LSP command becomes an MCP tool
    - This allows direct integration with Claude Code, Cursor, etc.
    - The CLI and MCP share the same core; MCP is just another frontend

---

## 5. Key Design Decisions

### One-shot vs Daemon

The MVP should be **one-shot**: each command spawns the server, sends the
request, gets the response, shuts down. This is simpler, stateless, and
debuggable.

The downside is startup latency. gopls takes ~1-3 seconds to initialize on a
medium project. For an agent making multiple queries, this adds up.

**Mitigation path:** Add daemon mode in Phase 4. The CLI transparently checks
for a running daemon and connects to it; if none exists, falls back to one-shot.
This is exactly how gopls works.

### Output Format

Default to human-readable, grep-compatible output. Always support `--json`.

The agent (or its tool harness) should use `--json` for reliable parsing, but
the text format is valuable for:
- Human debugging
- Piping into other Unix tools
- Quick manual testing

### Error Handling

LSP servers can be flaky. The CLI must handle:
- Server failing to start (bad binary, missing dependencies)
- Server timeout during initialization
- Server crash mid-request
- Requests that return empty/null results (symbol not found)

All errors should go to stderr. Exit codes: 0 = success, 1 = error, 2 = not found.

### File Synchronization

LSP servers work with "open documents." For a CLI that operates on files from
disk, the flow is:
1. Read the file from disk
2. Send `textDocument/didOpen` with the file contents
3. Send the actual LSP request
4. The server responds using its in-memory state

For diagnostics after edits, we'd need to send `didChange` or re-open with the
new content. The one-shot model handles this naturally: each invocation reads the
current file from disk.

---

## 6. Comparison: Build Cost vs Value

| Component | Effort | Agent Value |
|-----------|--------|-------------|
| Transport + lifecycle | Medium (most complex part) | Foundation for everything |
| `definition` | Small | **Very High** — eliminates ambiguous grep-for-definition |
| `references` | Small | **Very High** — precise call-site finding |
| `hover` | Small | **High** — instant type/signature info |
| `symbols` | Small | **High** — file structure without reading entire file |
| `diagnostics` | Medium | **Very High** — error checking without full build |
| `implementations` | Small | **High** — interface→impl navigation |
| `workspace-symbols` | Small | **Medium** — fuzzy project-wide search |
| `rename` | Medium | **Medium** — safe refactoring (agent can also do this with precision from references) |
| Daemon mode | Medium | **High** — makes repeated queries fast |
| MCP bridge | Medium | **Very High** — direct agent integration |

**The MVP sweet spot is:** transport + lifecycle + definition + references + hover
+ symbols + diagnostics. This covers ~90% of agent navigation needs with
moderate implementation effort.

---

## 7. Getting Started: First Implementation Steps

```bash
# 1. Initialize
mkdir -p cmd lsp output config
go mod init github.com/user/lsp-cli

# 2. Add dependencies
go get go.lsp.dev/protocol
go get go.lsp.dev/jsonrpc2
go get go.lsp.dev/uri
go get github.com/spf13/cobra

# 3. Build order:
#    lsp/transport.go  → JSON-RPC over stdio
#    lsp/client.go     → Initialize/shutdown lifecycle
#    lsp/uri.go        → Path ↔ URI conversion
#    lsp/document.go   → didOpen helper
#    cmd/root.go       → CLI skeleton
#    cmd/definition.go → First working command
#    cmd/hover.go      → Second command (validates the pattern)
#    → at this point the architecture is proven
#    → remaining commands are variations on the same pattern
```

The entire transport + lifecycle layer is ~300-400 lines of Go. Each command is
~50-100 lines. A working MVP with 5 commands is roughly **800-1200 lines of Go**
— a very achievable target for a speedrun session.
