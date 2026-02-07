# Evaluation: agent-cli-tools impact on coding tasks

## Goal

Measure whether `e` and `lsp-cli` improve AI coding agent performance compared to built-in tools (Read, Edit, Grep, Glob), and gather the agent's own qualitative feedback.

## Task

**Repo:** `charmbracelet/bubbletea`

**Task prompt (identical across all runs):**

> Add a `WindowCloseMsg` that gets dispatched when the terminal receives SIGHUP. Follow the existing pattern used by `WindowSizeMsg`. The new message type should be exported, and the signal handler should be registered alongside the existing signal handling. Include it in the standard program loop.

This task requires:
- Finding where `WindowSizeMsg` is defined and dispatched
- Tracing signal handling across files
- Understanding the `Msg` interface and `Cmd` pattern
- Following references across packages
- Making edits across 2-3 files

## Runs

### Run A — Control (vanilla Claude Code)

No extra tools. Agent uses built-in Read, Edit, Grep, Glob, Bash.

**Setup:** Fresh clone, no CLAUDE.md additions, no hooks.

**Prompt:**
```
Add a `WindowCloseMsg` that gets dispatched when the terminal receives SIGHUP.
Follow the existing pattern used by `WindowSizeMsg`. The new message type should
be exported, and the signal handler should be registered alongside the existing
signal handling. Include it in the standard program loop.
```

### Run B — Treatment (lsp-cli available)

Agent has lsp-cli available and is instructed to use it.

**Setup:** Fresh clone, add to CLAUDE.md:
```md
## Tools on PATH

### `lsp-cli` — code intelligence
Use `lsp-cli` for semantic code navigation — it's faster and more precise than grep for finding definitions, references, and understanding types.
- `lsp-cli definition <file:line:col>` — go to definition
- `lsp-cli references <file:line:col>` — find all references
- `lsp-cli hover <file:line:col>` — type info + docs
- `lsp-cli symbols <file>` — list symbols in file
- `lsp-cli diagnostics <file>` — errors/warnings without building
Locations are 1-indexed (file:line:col). Requires `gopls` on PATH.
```

**Prompt:** Same task prompt as Run A.

### Run C — Self-comparison (default first, then CLIs)

Agent does the task twice and gives feedback. Order: built-in tools → new CLIs.

**Setup:** Same as Run B (lsp-cli available + CLAUDE.md).

**Prompt:**
```
We're working on developer experience for AI coding agent tooling. We've received
feedback that CLI tools sometimes feel more natural to agents than built-in tool
calls. We'd like your help evaluating this.

Task: Add a `WindowCloseMsg` that gets dispatched when the terminal receives
SIGHUP. Follow the existing pattern used by `WindowSizeMsg`. The new message type
should be exported, and the signal handler should be registered alongside the
existing signal handling. Include it in the standard program loop.

Please:
1. Complete the task using ONLY the default built-in tools (Read, Edit, Grep, Glob).
   Do NOT use lsp-cli or e. Commit when done.
2. Revert that commit.
3. Redo the same task, but this time use `lsp-cli` for code navigation and `e`
   for edits where appropriate. Commit when done.
4. Write a brief comparison:
   - Which approach felt more efficient for navigating the codebase? Why?
   - Where did lsp-cli help most vs where was grep sufficient?
   - Where did `e` help vs the built-in Edit tool?
   - Any friction points with the CLI tools?
   - Overall preference and recommendation?
```

### Run D — Self-comparison (CLIs first, then default)

Same as Run C but reversed order to control for learning effects.

**Setup:** Same as Run B (lsp-cli available + CLAUDE.md).

**Prompt:**
```
We're working on developer experience for AI coding agent tooling. We've received
feedback that CLI tools sometimes feel more natural to agents than built-in tool
calls. We'd like your help evaluating this.

Task: Add a `WindowCloseMsg` that gets dispatched when the terminal receives
SIGHUP. Follow the existing pattern used by `WindowSizeMsg`. The new message type
should be exported, and the signal handler should be registered alongside the
existing signal handling. Include it in the standard program loop.

Please:
1. Complete the task using `lsp-cli` for code navigation and `e` for edits where
   appropriate. Use these CLI tools instead of the built-in equivalents. Commit
   when done.
2. Revert that commit.
3. Redo the same task using ONLY the default built-in tools (Read, Edit, Grep,
   Glob). Do NOT use lsp-cli or e. Commit when done.
4. Write a brief comparison:
   - Which approach felt more efficient for navigating the codebase? Why?
   - Where did lsp-cli help most vs where was grep sufficient?
   - Where did `e` help vs the built-in Edit tool?
   - Any friction points with the CLI tools?
   - Overall preference and recommendation?
```

## Metrics to compare

### Quantitative
- **Tool call count** — total calls per approach
- **Navigation calls** — grep/glob/read vs lsp-cli calls to reach the same understanding
- **Time to completion** — wall clock per approach
- **Correctness** — does the implementation compile and match the spec?
- **Edit accuracy** — number of failed/retried edits

### Qualitative (from Runs C and D)
- Agent's self-reported preference
- Identified strengths/weaknesses of each approach
- Order effects — does doing the task second (with prior knowledge) skew feedback?

## Running the evaluation

```bash
# Setup
git clone https://github.com/charmbracelet/bubbletea /tmp/eval-bubbletea
cd /tmp/eval-bubbletea

# Ensure gopls is available
go install golang.org/x/tools/gopls@latest

# Run A: vanilla
git checkout -b eval-run-a
# Start Claude Code, paste Run A prompt

# Run B: lsp-cli
git checkout main && git checkout -b eval-run-b
# Add CLAUDE.md with lsp-cli instructions, start Claude Code, paste Run B prompt

# Run C: default-first
git checkout main && git checkout -b eval-run-c
# Add CLAUDE.md, start Claude Code, paste Run C prompt

# Run D: cli-first
git checkout main && git checkout -b eval-run-d
# Add CLAUDE.md, start Claude Code, paste Run D prompt
```

After all runs, compare the conversation transcripts side by side.

## Results (2026-02-07)

Evaluated using Claude Opus 4.6 subagents within a single Claude Code session. All runs used the same bubbletea commit (`f9233d5`) and produced correct, building implementations.

### Quantitative

| Run | Approach | Tool calls | Tokens | Duration | Build |
|-----|----------|-----------|--------|----------|-------|
| A | Vanilla (control) | 22 | 39k | 110s | OK |
| B | lsp-cli | 38 | 32k | 192s | OK |
| C | Default → CLIs (self-comparison) | 68 | 50k | 372s | OK |
| D | CLIs → Default (self-comparison) | 71 | 40k | 379s | OK |

**Notes on A vs B:**
- Run A used fewer tool calls (22 vs 38) because each Grep/Read is one call, while lsp-cli requires a Bash call per invocation.
- Run B used fewer tokens (32k vs 39k) — lsp-cli returns compact, structured results vs full file reads.
- Run B was slower wall-clock (192s vs 110s) due to per-invocation gopls startup cost (~3s each).

### Qualitative (agent self-reports from Runs C and D)

Both agents arrived at the same conclusions despite reversed execution order, indicating the findings are robust and not an artifact of second-pass familiarity.

#### lsp-cli strengths

- **`symbols` was the standout command.** Both agents highlighted it as the best way to get a structural overview of large files (tea.go is 900+ lines) without reading the entire file. It immediately shows method names and line numbers.
- **`references` was precise.** Unlike grep, it returns only actual code references — no false positives from comments, documentation, or string literals. Both agents found this most valuable for tracing dispatch patterns.
- **`diagnostics` was useful as a fast pre-check** before running a full `go build ./...`.
- **`hover` provided type signatures** without reading the full function body.

#### lsp-cli weaknesses

- **Per-invocation startup cost is the primary friction.** Each call spawns a fresh gopls instance (~3s). Over 10+ calls this adds up significantly. A daemon/keep-alive mode would eliminate this entirely.
- **Requires `file:line:col` coordinates** for definition/references/hover — you often need to inspect the file first to find the column, adding an extra step.
- **Still need Read for implementation details.** lsp-cli shows *where* things are but not the surrounding logic. The combination of lsp-cli for navigation + Read for context worked well.

#### `e` editor weaknesses

- **Single-line matching limitation.** `e after` and `e before` match single lines only. Go code patterns frequently span multiple lines (struct definitions, case blocks), forcing fallback to line-number-based `e append` or the built-in Edit tool.
- **Line numbers shift after edits.** Each insertion/deletion invalidates subsequent line numbers, requiring re-inspection with `e show` or `lsp-cli symbols`. The built-in Edit tool's exact-string-match approach avoids this entirely.
- **Both agents unanimously preferred built-in Edit** for making code changes. Edit's multi-line exact-string-match is more robust and requires fewer round-trips.

#### Consensus recommendation

**Hybrid approach is best:**
- Use `lsp-cli symbols` for structural overviews of large files
- Use `lsp-cli references` for precise symbol tracing (especially during refactoring)
- Use `lsp-cli diagnostics` for fast error checking
- Use built-in Edit for all code modifications (not `e`)
- Use `e show <file> <range>` as a lightweight file viewer

### Identified improvements

1. **lsp-cli daemon mode** — Keep gopls alive between invocations. This is the single highest-impact improvement. Would reduce per-call latency from ~3s to near-zero, making lsp-cli strictly faster than grep for semantic queries. See [issue #1](https://github.com/c3d4r/agent-cli-tools/issues/1).
2. **`e` multi-line pattern support** — Allow `e after`, `e before`, and `e replace` to match patterns spanning multiple lines. Without this, `e` cannot compete with the built-in Edit tool for real-world code changes. See [issue #2](https://github.com/c3d4r/agent-cli-tools/issues/2).
