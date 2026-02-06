// e — a modern non-visual file editor for terminals and agents.
//
// Usage:
//
//	e <command> <file> [args...]
//
// Line-addressed commands:
//
//	e set       <file> <line> <text>          Replace a single line
//	e setrange  <file> <from>-<to> <text>     Replace a range of lines
//	e delete    <file> <line|from-to>         Delete line(s)
//	e insert    <file> <line> <text>          Insert before line
//	e append    <file> <line> <text>          Insert after line
//
// Content-addressed commands:
//
//	e replace   <file> <old> <new>            Exact string replace (first match)
//	e after     <file> <match> <text>         Insert text after matching line
//	e before    <file> <match> <text>         Insert text before matching line
//
// Other:
//
//	e show      <file> [from-to]              Show file with line numbers
//
// Flags:
//
//	--all       Replace/match all occurrences (not just first)
//	--regex     Treat match strings as regex
//	--dry-run   Preview changes without writing
//	--diff      Show unified diff of changes
//	--stdin     Read text argument from stdin (for multiline content)
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// flags
var (
	flagAll    bool
	flagRegex  bool
	flagDryRun bool
	flagDiff   bool
	flagStdin  bool
)

func main() {
	args := parseFlags(os.Args[1:])

	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	command := args[0]
	cmdArgs := args[1:]

	var err error
	switch command {
	case "set":
		err = cmdSet(cmdArgs)
	case "setrange":
		err = cmdSetRange(cmdArgs)
	case "delete", "del":
		err = cmdDelete(cmdArgs)
	case "insert", "ins":
		err = cmdInsert(cmdArgs)
	case "append", "app":
		err = cmdAppend(cmdArgs)
	case "replace", "rep":
		err = cmdReplace(cmdArgs)
	case "after":
		err = cmdAfter(cmdArgs)
	case "before":
		err = cmdBefore(cmdArgs)
	case "show":
		err = cmdShow(cmdArgs)
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `e — modern non-visual file editor for terminals and agents

Usage: e <command> <file> [args...]

Line-addressed commands:
  set       <file> <line> <text>          Replace a single line
  setrange  <file> <from>-<to> <text>     Replace a range of lines
  delete    <file> <line|from-to>         Delete line(s)
  insert    <file> <line> <text>          Insert before line
  append    <file> <line> <text>          Insert after line

Content-addressed commands:
  replace   <file> <old> <new>            Exact string replace (first match)
  after     <file> <match> <text>         Insert text after matching line
  before    <file> <match> <text>         Insert text before matching line

Other:
  show      <file> [from-to]             Show file with line numbers

Flags:
  --all       Replace/match all occurrences (not just first)
  --regex     Treat match strings as regex
  --dry-run   Preview changes without writing
  --diff      Show unified diff of changes
  --stdin     Read text argument from stdin (for multiline content)

Examples:
  e set main.go 42 "    return nil"
  e delete main.go 10-15
  e insert main.go 1 "// Copyright 2025"
  e replace main.go 'func Foo()' 'func Bar(ctx context.Context)'
  e after main.go 'import (' '    "context"'
  e show main.go 40-50
  echo -e "line1\nline2" | e --stdin insert main.go 5
  e --diff replace main.go 'oldFunc' 'newFunc'
`)
}

// parseFlags extracts flags from args and returns remaining positional args.
func parseFlags(args []string) []string {
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--all":
			flagAll = true
		case "--regex":
			flagRegex = true
		case "--dry-run":
			flagDryRun = true
		case "--diff":
			flagDiff = true
			flagDryRun = true // --diff implies dry-run
		case "--stdin":
			flagStdin = true
		default:
			positional = append(positional, arg)
		}
	}
	return positional
}

// --- File I/O helpers ---

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	// Preserve trailing newline behavior
	if strings.HasSuffix(text, "\n") {
		text = text[:len(text)-1]
	}
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

func writeResult(path string, original, modified []string) error {
	if flagDiff {
		printDiff(path, original, modified)
		return nil
	}
	if flagDryRun {
		// Print the modified content with line numbers
		for i, line := range modified {
			fmt.Fprintf(os.Stdout, "%4d\t%s\n", i+1, line)
		}
		return nil
	}
	return writeLines(path, modified)
}

// readStdin reads all of stdin and returns it as the text argument.
func readStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	s := string(data)
	// Trim single trailing newline (shells add one)
	s = strings.TrimSuffix(s, "\n")
	return s, nil
}

// getText returns the text arg from args or stdin.
func getText(args []string, index int) (string, error) {
	if flagStdin {
		return readStdin()
	}
	if index >= len(args) {
		return "", fmt.Errorf("missing text argument (use --stdin for multiline)")
	}
	return args[index], nil
}

// --- Range parsing ---

// parseRange parses "N" or "N-M" into 1-indexed start, end.
func parseRange(s string) (start, end int, err error) {
	if idx := strings.Index(s, "-"); idx >= 0 {
		start, err = strconv.Atoi(s[:idx])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range start %q: %w", s[:idx], err)
		}
		end, err = strconv.Atoi(s[idx+1:])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range end %q: %w", s[idx+1:], err)
		}
	} else {
		start, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid line number %q: %w", s, err)
		}
		end = start
	}
	if start < 1 || end < start {
		return 0, 0, fmt.Errorf("invalid range %d-%d (lines are 1-indexed)", start, end)
	}
	return start, end, nil
}

func validateLine(line int, total int) error {
	if line < 1 || line > total {
		return fmt.Errorf("line %d out of range (file has %d lines)", line, total)
	}
	return nil
}

// --- Diff ---

func printDiff(path string, old, new []string) {
	fmt.Fprintf(os.Stdout, "--- %s\n", path)
	fmt.Fprintf(os.Stdout, "+++ %s\n", path)

	// Simple line-by-line diff with context
	type change struct {
		oldStart, oldEnd int // 0-indexed, exclusive end
		newStart, newEnd int
	}

	var changes []change
	oi, ni := 0, 0
	for oi < len(old) && ni < len(new) {
		if old[oi] == new[ni] {
			oi++
			ni++
			continue
		}
		// Found a difference — find the extent
		cs := change{oldStart: oi, newStart: ni}
		// Scan forward to find where they sync up again
		found := false
		for scanLen := 1; scanLen < len(old)-oi+len(new)-ni+1; scanLen++ {
			for oo := 0; oo <= scanLen && oi+oo <= len(old); oo++ {
				nn := scanLen - oo
				if ni+nn > len(new) {
					continue
				}
				if oi+oo < len(old) && ni+nn < len(new) && old[oi+oo] == new[ni+nn] {
					cs.oldEnd = oi + oo
					cs.newEnd = ni + nn
					changes = append(changes, cs)
					oi = oi + oo
					ni = ni + nn
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			cs.oldEnd = len(old)
			cs.newEnd = len(new)
			changes = append(changes, cs)
			oi = len(old)
			ni = len(new)
		}
	}
	// Handle trailing content
	if oi < len(old) || ni < len(new) {
		changes = append(changes, change{
			oldStart: oi, oldEnd: len(old),
			newStart: ni, newEnd: len(new),
		})
	}

	// Print hunks with context
	ctx := 3
	for _, c := range changes {
		// Context lines before
		ctxStart := c.oldStart - ctx
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxEnd := c.oldEnd + ctx
		if ctxEnd > len(old) {
			ctxEnd = len(old)
		}
		newCtxEnd := c.newEnd + ctx
		if newCtxEnd > len(new) {
			newCtxEnd = len(new)
		}

		fmt.Fprintf(os.Stdout, "@@ -%d,%d +%d,%d @@\n",
			ctxStart+1, ctxEnd-ctxStart,
			c.newStart-(c.oldStart-ctxStart)+1, (newCtxEnd)-(c.newStart-(c.oldStart-ctxStart)))

		// Context before
		for i := ctxStart; i < c.oldStart; i++ {
			fmt.Fprintf(os.Stdout, " %s\n", old[i])
		}
		// Removed lines
		for i := c.oldStart; i < c.oldEnd; i++ {
			fmt.Fprintf(os.Stdout, "-%s\n", old[i])
		}
		// Added lines
		for i := c.newStart; i < c.newEnd; i++ {
			fmt.Fprintf(os.Stdout, "+%s\n", new[i])
		}
		// Context after
		for i := c.oldEnd; i < ctxEnd; i++ {
			fmt.Fprintf(os.Stdout, " %s\n", old[i])
		}
	}
}

// --- Line-addressed commands ---

func cmdSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e set <file> <line> <text>")
	}
	path := args[0]
	lineNum, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid line number: %w", err)
	}
	text, err := getText(args, 2)
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	if err := validateLine(lineNum, len(lines)); err != nil {
		return err
	}

	original := copyLines(lines)
	lines[lineNum-1] = text
	return writeResult(path, original, lines)
}

func cmdSetRange(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e setrange <file> <from>-<to> <text>")
	}
	path := args[0]
	start, end, err := parseRange(args[1])
	if err != nil {
		return err
	}
	text, err := getText(args, 2)
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	if err := validateLine(start, len(lines)); err != nil {
		return err
	}
	if err := validateLine(end, len(lines)); err != nil {
		return err
	}

	original := copyLines(lines)
	newTextLines := strings.Split(text, "\n")
	modified := make([]string, 0, len(lines)-( end-start+1)+len(newTextLines))
	modified = append(modified, lines[:start-1]...)
	modified = append(modified, newTextLines...)
	modified = append(modified, lines[end:]...)

	return writeResult(path, original, modified)
}

func cmdDelete(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: e delete <file> <line|from-to>")
	}
	path := args[0]
	start, end, err := parseRange(args[1])
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	if err := validateLine(start, len(lines)); err != nil {
		return err
	}
	if err := validateLine(end, len(lines)); err != nil {
		return err
	}

	original := copyLines(lines)
	modified := make([]string, 0, len(lines)-(end-start+1))
	modified = append(modified, lines[:start-1]...)
	modified = append(modified, lines[end:]...)

	return writeResult(path, original, modified)
}

func cmdInsert(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e insert <file> <line> <text>")
	}
	path := args[0]
	lineNum, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid line number: %w", err)
	}
	text, err := getText(args, 2)
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	if lineNum < 1 || lineNum > len(lines)+1 {
		return fmt.Errorf("line %d out of range (file has %d lines, insert accepts 1-%d)", lineNum, len(lines), len(lines)+1)
	}

	original := copyLines(lines)
	newTextLines := strings.Split(text, "\n")
	modified := make([]string, 0, len(lines)+len(newTextLines))
	modified = append(modified, lines[:lineNum-1]...)
	modified = append(modified, newTextLines...)
	modified = append(modified, lines[lineNum-1:]...)

	return writeResult(path, original, modified)
}

func cmdAppend(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e append <file> <line> <text>")
	}
	path := args[0]
	lineNum, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid line number: %w", err)
	}
	text, err := getText(args, 2)
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	if err := validateLine(lineNum, len(lines)); err != nil {
		return err
	}

	original := copyLines(lines)
	newTextLines := strings.Split(text, "\n")
	modified := make([]string, 0, len(lines)+len(newTextLines))
	modified = append(modified, lines[:lineNum]...)
	modified = append(modified, newTextLines...)
	modified = append(modified, lines[lineNum:]...)

	return writeResult(path, original, modified)
}

// --- Content-addressed commands ---

func cmdReplace(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e replace <file> <old> <new>")
	}
	path := args[0]
	oldText := args[1]
	newText := ""
	if !flagStdin {
		if len(args) < 3 {
			return fmt.Errorf("usage: e replace <file> <old> <new>")
		}
		newText = args[2]
	} else {
		var err error
		newText, err = readStdin()
		if err != nil {
			return err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	original := content

	if flagRegex {
		re, err := regexp.Compile(oldText)
		if err != nil {
			return fmt.Errorf("invalid regex %q: %w", oldText, err)
		}
		if flagAll {
			content = re.ReplaceAllString(content, newText)
		} else {
			loc := re.FindStringIndex(content)
			if loc == nil {
				return fmt.Errorf("pattern %q not found", oldText)
			}
			content = content[:loc[0]] + re.ReplaceAllString(content[loc[0]:loc[1]], newText) + content[loc[1]:]
		}
	} else {
		if !strings.Contains(content, oldText) {
			return fmt.Errorf("text %q not found", oldText)
		}
		if flagAll {
			content = strings.ReplaceAll(content, oldText, newText)
		} else {
			content = strings.Replace(content, oldText, newText, 1)
		}
	}

	if content == original {
		fmt.Fprintln(os.Stderr, "no changes")
		return nil
	}

	originalLines := toLines(original)
	modifiedLines := toLines(content)

	if flagDiff {
		printDiff(path, originalLines, modifiedLines)
		return nil
	}
	if flagDryRun {
		for i, line := range modifiedLines {
			fmt.Fprintf(os.Stdout, "%4d\t%s\n", i+1, line)
		}
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func cmdAfter(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e after <file> <match> <text>")
	}
	path := args[0]
	matchText := args[1]
	text, err := getText(args, 2)
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	original := copyLines(lines)

	matcher, err := newMatcher(matchText)
	if err != nil {
		return err
	}

	newTextLines := strings.Split(text, "\n")
	modified := make([]string, 0, len(lines)+len(newTextLines))
	matched := false
	for _, line := range lines {
		modified = append(modified, line)
		if matcher.matches(line) && (flagAll || !matched) {
			modified = append(modified, newTextLines...)
			matched = true
		}
	}

	if !matched {
		return fmt.Errorf("pattern %q not found", matchText)
	}

	return writeResult(path, original, modified)
}

func cmdBefore(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: e before <file> <match> <text>")
	}
	path := args[0]
	matchText := args[1]
	text, err := getText(args, 2)
	if err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	original := copyLines(lines)

	matcher, err := newMatcher(matchText)
	if err != nil {
		return err
	}

	newTextLines := strings.Split(text, "\n")
	modified := make([]string, 0, len(lines)+len(newTextLines))
	matched := false
	for _, line := range lines {
		if matcher.matches(line) && (flagAll || !matched) {
			modified = append(modified, newTextLines...)
			matched = true
		}
		modified = append(modified, line)
	}

	if !matched {
		return fmt.Errorf("pattern %q not found", matchText)
	}

	return writeResult(path, original, modified)
}

// --- Show command ---

func cmdShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: e show <file> [from-to]")
	}
	path := args[0]

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	start, end := 1, -1 // -1 means "to EOF"
	if len(args) >= 2 {
		s, e, err := parseRange(args[1])
		if err != nil {
			return err
		}
		start = s
		end = e
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < start {
			continue
		}
		if end > 0 && lineNum > end {
			break
		}
		fmt.Fprintf(os.Stdout, "%4d\t%s\n", lineNum, scanner.Text())
	}
	return scanner.Err()
}

// --- Helpers ---

func copyLines(lines []string) []string {
	cp := make([]string, len(lines))
	copy(cp, lines)
	return cp
}

func toLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// matcher wraps either a literal string match or a regex match.
type matcher struct {
	literal string
	re      *regexp.Regexp
}

func newMatcher(pattern string) (*matcher, error) {
	if flagRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		return &matcher{re: re}, nil
	}
	return &matcher{literal: pattern}, nil
}

func (m *matcher) matches(line string) bool {
	if m.re != nil {
		return m.re.MatchString(line)
	}
	return strings.Contains(line, m.literal)
}
