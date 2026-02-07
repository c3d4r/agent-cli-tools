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
