package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client manages an LSP server process and provides typed methods for LSP requests.
type Client struct {
	cmd     *exec.Cmd
	conn    *Conn
	rootURI string
	verbose bool

	// diagnostics collected from publishDiagnostics notifications
	diagMu      sync.Mutex
	diagnostics map[string][]Diagnostic // URI -> diagnostics
	diagCh      chan struct{}           // signaled when new diagnostics arrive

	// progress tracking for server readiness
	progressMu sync.Mutex
	progDone   chan struct{} // closed when server finishes initial loading
	progClosed bool
}

// StartClient spawns the language server and performs the initialize handshake.
func StartClient(serverCmd []string, rootDir string, verbose bool) (*Client, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	cmd := exec.Command(serverCmd[0], serverCmd[1:]...)
	if verbose {
		cmd.Stderr = os.Stderr
	}
	cmd.Dir = absRoot

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start server %q: %w", serverCmd[0], err)
	}

	transport := NewTransport(stdout, stdin)
	conn := NewConn(transport)

	c := &Client{
		cmd:         cmd,
		conn:        conn,
		rootURI:     fileURI(absRoot),
		verbose:     verbose,
		diagnostics: make(map[string][]Diagnostic),
		diagCh:      make(chan struct{}, 1),
		progDone:    make(chan struct{}),
	}

	// Handle server notifications
	conn.NotificationHandler = c.handleNotification

	// Initialize handshake
	if err := c.initialize(); err != nil {
		c.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	return c, nil
}

func (c *Client) initialize() error {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   c.rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Definition: &DefinitionClientCapabilities{
					LinkSupport: true,
				},
				References: &ReferencesClientCapabilities{},
				Hover: &HoverClientCapabilities{
					ContentFormat: []string{"plaintext", "markdown"},
				},
				DocumentSymbol: &DocumentSymbolClientCapabilities{
					HierarchicalDocumentSymbolSupport: true,
				},
				Implementation: &ImplementationClientCapabilities{
					LinkSupport: true,
				},
				PublishDiagnostics: &PublishDiagnosticsClientCapabilities{
					RelatedInformation: true,
				},
			},
		},
	}

	result, err := c.conn.Call("initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("unmarshal initialize result: %w", err)
	}

	// Send initialized notification
	if err := c.conn.Notify("initialized", struct{}{}); err != nil {
		return fmt.Errorf("initialized notification: %w", err)
	}

	return nil
}

func (c *Client) handleNotification(method string, params json.RawMessage) {
	if c.verbose {
		fmt.Fprintf(os.Stderr, "notification: %s\n", method)
	}

	switch method {
	case "textDocument/publishDiagnostics":
		var p PublishDiagnosticsParams
		if err := json.Unmarshal(params, &p); err != nil {
			return
		}
		c.diagMu.Lock()
		c.diagnostics[p.URI] = p.Diagnostics
		c.diagMu.Unlock()

		// Receiving diagnostics means the server has processed the file — signal ready
		c.signalReady()

		// Signal that new diagnostics arrived
		select {
		case c.diagCh <- struct{}{}:
		default:
		}

	case "$/progress":
		// Track work done progress — gopls uses this to signal loading completion
		var prog struct {
			Token string          `json:"token"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(params, &prog); err != nil {
			return
		}
		var val struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(prog.Value, &val); err != nil {
			return
		}
		if c.verbose {
			fmt.Fprintf(os.Stderr, "progress: token=%s kind=%s\n", prog.Token, val.Kind)
		}
		if val.Kind == "end" {
			c.signalReady()
		}

	case "window/workDoneProgress/create":
		// Server is asking to create a progress token — acknowledge it
		// (gopls sends this before $/progress)
	}
}

// signalReady marks the server as ready (initial loading complete).
func (c *Client) signalReady() {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	if !c.progClosed {
		c.progClosed = true
		close(c.progDone)
	}
}

// WaitReady blocks until the server signals it has finished initial loading,
// or until the timeout expires. Returns true if ready, false if timed out.
func (c *Client) WaitReady(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-c.progDone:
		return true
	case <-timer.C:
		return false
	}
}

// OpenFile sends textDocument/didOpen for the given file.
func (c *Client) OpenFile(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	uri := fileURI(absPath)
	langID := detectLanguageID(absPath)

	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: langID,
			Version:    1,
			Text:       string(content),
		},
	}

	if err := c.conn.Notify("textDocument/didOpen", params); err != nil {
		return "", fmt.Errorf("didOpen: %w", err)
	}

	return uri, nil
}

// CloseFile sends textDocument/didClose for the given URI.
func (c *Client) CloseFile(uri string) error {
	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}
	return c.conn.Notify("textDocument/didClose", params)
}

// Definition requests the definition of the symbol at the given position.
func (c *Client) Definition(uri string, line, col int) ([]Location, error) {
	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: col},
		},
	}

	if c.verbose {
		fmt.Fprintf(os.Stderr, "definition request: uri=%s line=%d col=%d\n", uri, line, col)
	}

	result, err := c.conn.Call("textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	if c.verbose {
		fmt.Fprintf(os.Stderr, "definition response: %s\n", string(result))
	}

	return parseLocationResponse(result)
}

// References requests all references to the symbol at the given position.
func (c *Client) References(uri string, line, col int, includeDecl bool) ([]Location, error) {
	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: col},
		},
		Context: ReferenceContext{
			IncludeDeclaration: includeDecl,
		},
	}

	result, err := c.conn.Call("textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locs []Location
	if err := json.Unmarshal(result, &locs); err != nil {
		return nil, fmt.Errorf("unmarshal references: %w", err)
	}
	return locs, nil
}

// Hover requests hover information at the given position.
func (c *Client) Hover(uri string, line, col int) (*Hover, error) {
	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: col},
		},
	}

	result, err := c.conn.Call("textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	if string(result) == "null" {
		return nil, nil
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, fmt.Errorf("unmarshal hover: %w", err)
	}
	return &hover, nil
}

// DocumentSymbols requests symbols in the given document.
// Returns (hierarchical, flat, error) — one of the two will be non-nil.
func (c *Client) DocumentSymbols(uri string) ([]DocumentSymbol, []SymbolInformation, error) {
	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	result, err := c.conn.Call("textDocument/documentSymbol", params)
	if err != nil {
		return nil, nil, err
	}

	// Try hierarchical first (DocumentSymbol[])
	var docSyms []DocumentSymbol
	if err := json.Unmarshal(result, &docSyms); err == nil && len(docSyms) > 0 {
		// Verify it's actually hierarchical by checking for Range field
		if docSyms[0].Range.End.Line > 0 || docSyms[0].Range.End.Character > 0 || docSyms[0].Name != "" {
			return docSyms, nil, nil
		}
	}

	// Fall back to flat (SymbolInformation[])
	var symInfos []SymbolInformation
	if err := json.Unmarshal(result, &symInfos); err != nil {
		return nil, nil, fmt.Errorf("unmarshal document symbols: %w", err)
	}
	return nil, symInfos, nil
}

// WorkspaceSymbols queries for symbols across the workspace.
func (c *Client) WorkspaceSymbols(query string) ([]SymbolInformation, error) {
	params := WorkspaceSymbolParams{Query: query}

	result, err := c.conn.Call("workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	var syms []SymbolInformation
	if err := json.Unmarshal(result, &syms); err != nil {
		return nil, fmt.Errorf("unmarshal workspace symbols: %w", err)
	}
	return syms, nil
}

// Implementations requests implementations of the symbol at the given position.
func (c *Client) Implementations(uri string, line, col int) ([]Location, error) {
	params := ImplementationParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     Position{Line: line, Character: col},
		},
	}

	result, err := c.conn.Call("textDocument/implementation", params)
	if err != nil {
		return nil, err
	}

	return parseLocationResponse(result)
}

// GetDiagnostics returns the most recently received diagnostics for a URI.
func (c *Client) GetDiagnostics(uri string) []Diagnostic {
	c.diagMu.Lock()
	defer c.diagMu.Unlock()
	return c.diagnostics[uri]
}

// WaitForDiagnostics waits for a diagnostics notification and returns.
// It returns immediately if diagnostics have already been received for the URI.
func (c *Client) WaitForDiagnostics(uri string) []Diagnostic {
	// Check if we already have diagnostics
	c.diagMu.Lock()
	if diags, ok := c.diagnostics[uri]; ok {
		c.diagMu.Unlock()
		return diags
	}
	c.diagMu.Unlock()

	// Wait for notification
	<-c.diagCh

	c.diagMu.Lock()
	defer c.diagMu.Unlock()
	return c.diagnostics[uri]
}

// DiagnosticsChannel returns a channel signaled when new diagnostics arrive.
func (c *Client) DiagnosticsChannel() <-chan struct{} {
	return c.diagCh
}

// AllDiagnostics returns all collected diagnostics keyed by URI.
func (c *Client) AllDiagnostics() map[string][]Diagnostic {
	c.diagMu.Lock()
	defer c.diagMu.Unlock()
	result := make(map[string][]Diagnostic, len(c.diagnostics))
	for k, v := range c.diagnostics {
		result[k] = v
	}
	return result
}

// Close shuts down the LSP server and cleans up.
func (c *Client) Close() error {
	// Send shutdown request
	c.conn.Call("shutdown", nil)
	// Send exit notification
	c.conn.Notify("exit", nil)
	c.conn.Close()

	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	return c.cmd.Wait()
}

// parseLocationResponse handles the various shapes of location responses:
// Location | Location[] | LocationLink[]
func parseLocationResponse(result json.RawMessage) ([]Location, error) {
	if string(result) == "null" {
		return nil, nil
	}

	// Try Location[] first
	var locs []Location
	if err := json.Unmarshal(result, &locs); err == nil {
		return locs, nil
	}

	// Try single Location
	var loc Location
	if err := json.Unmarshal(result, &loc); err == nil && loc.URI != "" {
		return []Location{loc}, nil
	}

	// Try LocationLink[]
	var links []LocationLink
	if err := json.Unmarshal(result, &links); err == nil {
		locs := make([]Location, len(links))
		for i, link := range links {
			locs[i] = Location{
				URI:   link.TargetURI,
				Range: link.TargetSelectionRange,
			}
		}
		return locs, nil
	}

	return nil, fmt.Errorf("unexpected definition response shape: %s", string(result))
}

// fileURI converts a filesystem path to a file:// URI.
func fileURI(path string) string {
	absPath, _ := filepath.Abs(path)
	return "file://" + absPath
}

// URIToPath converts a file:// URI back to a filesystem path.
func URIToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

// detectLanguageID guesses the LSP language ID from a file extension.
func detectLanguageID(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".jsx":
		return "javascriptreact"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".cs":
		return "csharp"
	case ".lua":
		return "lua"
	case ".sh", ".bash":
		return "shellscript"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	default:
		return "plaintext"
	}
}
