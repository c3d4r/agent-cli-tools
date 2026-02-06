// lsp-cli — A command-line LSP client for AI coding agents.
//
// Usage:
//
//	lsp-cli [flags] <command> <args>
//
// Commands:
//
//	definition  <file:line:col>           Find definition of symbol
//	references  <file:line:col>           Find all references to symbol
//	hover       <file:line:col>           Show type/docs for symbol
//	symbols     <file>                    List symbols in file
//	diagnostics <file> [file...]          Show diagnostics (errors/warnings)
//	implementations <file:line:col>       Find implementations of interface
//	workspace-symbols <query>             Search symbols across workspace
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/c3d4r/cli-lsp-mcp/internal/config"
	"github.com/c3d4r/cli-lsp-mcp/internal/lsp"
	"github.com/c3d4r/cli-lsp-mcp/internal/output"
)

var (
	flagJSON    bool
	flagServer  string
	flagRoot    string
	flagVerbose bool
	flagTimeout int
)

func init() {
	flag.BoolVar(&flagJSON, "json", false, "output as JSON")
	flag.StringVar(&flagServer, "server", "", "language server command (overrides auto-detect)")
	flag.StringVar(&flagRoot, "root", "", "workspace root directory (default: auto-detect from file)")
	flag.BoolVar(&flagVerbose, "v", false, "verbose output (show server stderr)")
	flag.IntVar(&flagTimeout, "timeout", 30, "timeout in seconds for server operations")
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	command := args[0]
	cmdArgs := args[1:]

	var err error
	switch command {
	case "definition", "def":
		err = cmdDefinition(cmdArgs)
	case "references", "refs":
		err = cmdReferences(cmdArgs)
	case "hover":
		err = cmdHover(cmdArgs)
	case "symbols", "syms":
		err = cmdSymbols(cmdArgs)
	case "diagnostics", "diag":
		err = cmdDiagnostics(cmdArgs)
	case "implementations", "impl":
		err = cmdImplementations(cmdArgs)
	case "workspace-symbols", "wsyms":
		err = cmdWorkspaceSymbols(cmdArgs)
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
	fmt.Fprintf(os.Stderr, `lsp-cli — CLI LSP client for AI coding agents

Usage: lsp-cli [flags] <command> <args>

Commands:
  definition  <file:line:col>           Find definition of symbol
  references  <file:line:col>           Find all references to symbol
  hover       <file:line:col>           Show type/docs for symbol
  symbols     <file>                    List symbols in file
  diagnostics <file> [file...]          Show diagnostics (errors/warnings)
  implementations <file:line:col>       Find implementations of interface
  workspace-symbols <query>             Search symbols across workspace

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Location format: file.go:line:col (1-indexed, like compiler output)

Examples:
  lsp-cli definition ./server/handler.go:42:15
  lsp-cli references ./pkg/auth/token.go:28:6
  lsp-cli hover ./server/handler.go:42:15
  lsp-cli symbols ./server/handler.go
  lsp-cli diagnostics ./server/handler.go
  lsp-cli --json definition ./server/handler.go:42:15
`)
}

// parseLocation parses "file.go:line:col" into (file, line-0indexed, col-0indexed).
func parseLocation(s string) (file string, line, col int, err error) {
	// Split from the right to handle paths with colons (Windows, etc.)
	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return "", 0, 0, fmt.Errorf("expected file:line:col, got %q", s)
	}

	col, err = strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid column %q: %w", parts[len(parts)-1], err)
	}
	line, err = strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid line %q: %w", parts[len(parts)-2], err)
	}

	file = strings.Join(parts[:len(parts)-2], ":")

	// Convert from 1-indexed (human) to 0-indexed (LSP)
	line--
	col--
	if line < 0 || col < 0 {
		return "", 0, 0, fmt.Errorf("line and column must be >= 1")
	}

	return file, line, col, nil
}

// resolveRoot determines the workspace root directory.
func resolveRoot(filePath string) string {
	if flagRoot != "" {
		abs, err := filepath.Abs(flagRoot)
		if err == nil {
			return abs
		}
		return flagRoot
	}

	// Walk up from the file looking for project markers
	dir, err := filepath.Abs(filepath.Dir(filePath))
	if err != nil {
		return "."
	}

	markers := []string{"go.mod", "go.sum", "Cargo.toml", "package.json", "pyproject.toml", "setup.py", ".git"}
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback to file's directory
	abs, _ := filepath.Abs(filepath.Dir(filePath))
	return abs
}

// startClient creates an LSP client for the given file.
func startClient(filePath string) (*lsp.Client, error) {
	var serverCmd []string

	if flagServer != "" {
		serverCmd = config.ParseServerFlag(flagServer)
	} else {
		cfg, err := config.DetectServer(filePath)
		if err != nil {
			return nil, err
		}
		serverCmd = cfg.Command
	}

	root := resolveRoot(filePath)

	if flagVerbose {
		fmt.Fprintf(os.Stderr, "server: %v\n", serverCmd)
		fmt.Fprintf(os.Stderr, "root: %s\n", root)
	}

	client, err := lsp.StartClient(serverCmd, root, flagVerbose)
	if err != nil {
		return nil, fmt.Errorf("start LSP server: %w", err)
	}

	return client, nil
}

// openAndWait opens a file and waits for the server to be ready.
func openAndWait(client *lsp.Client, file string) (string, error) {
	uri, err := client.OpenFile(file)
	if err != nil {
		return "", err
	}

	// Wait for server to finish loading (progress end), with timeout fallback.
	timeout := time.Duration(flagTimeout) * time.Second
	if !client.WaitReady(timeout) {
		if flagVerbose {
			fmt.Fprintln(os.Stderr, "warning: timed out waiting for server ready")
		}
	}

	return uri, nil
}

func formatter() *output.Formatter {
	return &output.Formatter{
		Writer: os.Stdout,
		JSON:   flagJSON,
	}
}

func cmdDefinition(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lsp-cli definition <file:line:col>")
	}

	file, line, col, err := parseLocation(args[0])
	if err != nil {
		return err
	}

	client, err := startClient(file)
	if err != nil {
		return err
	}
	defer client.Close()

	uri, err := openAndWait(client, file)
	if err != nil {
		return err
	}

	locs, err := client.Definition(uri, line, col)
	if err != nil {
		return fmt.Errorf("definition: %w", err)
	}

	if len(locs) == 0 {
		fmt.Fprintln(os.Stderr, "no definition found")
		os.Exit(2)
	}

	return formatter().Locations(locs)
}

func cmdReferences(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lsp-cli references <file:line:col>")
	}

	file, line, col, err := parseLocation(args[0])
	if err != nil {
		return err
	}

	client, err := startClient(file)
	if err != nil {
		return err
	}
	defer client.Close()

	uri, err := openAndWait(client, file)
	if err != nil {
		return err
	}

	locs, err := client.References(uri, line, col, true)
	if err != nil {
		return fmt.Errorf("references: %w", err)
	}

	if len(locs) == 0 {
		fmt.Fprintln(os.Stderr, "no references found")
		os.Exit(2)
	}

	return formatter().Locations(locs)
}

func cmdHover(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lsp-cli hover <file:line:col>")
	}

	file, line, col, err := parseLocation(args[0])
	if err != nil {
		return err
	}

	client, err := startClient(file)
	if err != nil {
		return err
	}
	defer client.Close()

	uri, err := openAndWait(client, file)
	if err != nil {
		return err
	}

	hover, err := client.Hover(uri, line, col)
	if err != nil {
		return fmt.Errorf("hover: %w", err)
	}

	if hover == nil {
		fmt.Fprintln(os.Stderr, "no hover information")
		os.Exit(2)
	}

	return formatter().Hover(hover)
}

func cmdSymbols(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lsp-cli symbols <file>")
	}

	file := args[0]

	client, err := startClient(file)
	if err != nil {
		return err
	}
	defer client.Close()

	uri, err := openAndWait(client, file)
	if err != nil {
		return err
	}

	docSyms, symInfos, err := client.DocumentSymbols(uri)
	if err != nil {
		return fmt.Errorf("symbols: %w", err)
	}

	f := formatter()
	if docSyms != nil {
		return f.DocumentSymbols(docSyms)
	}
	return f.SymbolInformations(symInfos)
}

func cmdDiagnostics(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lsp-cli diagnostics <file> [file...]")
	}

	client, err := startClient(args[0])
	if err != nil {
		return err
	}
	defer client.Close()

	// Open all files
	uris := make([]string, len(args))
	for i, file := range args {
		uri, err := client.OpenFile(file)
		if err != nil {
			return err
		}
		uris[i] = uri
	}

	// Wait for server to be ready, then give extra time for diagnostics
	timeout := time.Duration(flagTimeout) * time.Second
	client.WaitReady(timeout)

	// Give a bit more time for diagnostics to arrive after loading
	time.Sleep(500 * time.Millisecond)

	f := formatter()
	allDiags := client.AllDiagnostics()

	hasOutput := false
	for _, uri := range uris {
		if diags, ok := allDiags[uri]; ok && len(diags) > 0 {
			f.Diagnostics(uri, diags)
			hasOutput = true
		}
	}

	if !hasOutput {
		if flagJSON {
			fmt.Fprintln(os.Stdout, "[]")
		}
	}

	return nil
}

func cmdImplementations(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lsp-cli implementations <file:line:col>")
	}

	file, line, col, err := parseLocation(args[0])
	if err != nil {
		return err
	}

	client, err := startClient(file)
	if err != nil {
		return err
	}
	defer client.Close()

	uri, err := openAndWait(client, file)
	if err != nil {
		return err
	}

	locs, err := client.Implementations(uri, line, col)
	if err != nil {
		return fmt.Errorf("implementations: %w", err)
	}

	if len(locs) == 0 {
		fmt.Fprintln(os.Stderr, "no implementations found")
		os.Exit(2)
	}

	return formatter().Locations(locs)
}

func cmdWorkspaceSymbols(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lsp-cli workspace-symbols <query>")
	}

	query := args[0]

	root := flagRoot
	if root == "" {
		root = "."
	}

	// Find any source file in the root to detect the server
	refFile, err := findAnySourceFile(root)
	if err != nil {
		return fmt.Errorf("cannot find source file for server detection: %w", err)
	}

	client, err := startClient(refFile)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = openAndWait(client, refFile)
	if err != nil {
		return err
	}

	syms, err := client.WorkspaceSymbols(query)
	if err != nil {
		return fmt.Errorf("workspace symbols: %w", err)
	}

	if len(syms) == 0 {
		fmt.Fprintln(os.Stderr, "no symbols found")
		os.Exit(2)
	}

	return formatter().SymbolInformations(syms)
}

// findAnySourceFile walks the directory to find any recognized source file.
func findAnySourceFile(dir string) (string, error) {
	exts := []string{".go", ".py", ".ts", ".js", ".rs", ".java", ".c", ".cpp", ".rb"}

	var found string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" || info.IsDir() {
			if found != "" {
				return filepath.SkipAll
			}
			return nil
		}
		ext := filepath.Ext(path)
		for _, e := range exts {
			if ext == e {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	if found == "" {
		return "", fmt.Errorf("no source files found in %s", dir)
	}
	return found, nil
}
