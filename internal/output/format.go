// Package output handles formatting LSP results for display.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/c3d4r/agent-cli-tools/internal/lsp"
)

// Formatter controls how results are displayed.
type Formatter struct {
	Writer io.Writer
	JSON   bool
}

// Locations prints a list of locations.
func (f *Formatter) Locations(locs []lsp.Location) error {
	if f.JSON {
		return f.writeJSON(locs)
	}
	for _, loc := range locs {
		path := lsp.URIToPath(loc.URI)
		fmt.Fprintf(f.Writer, "%s:%d:%d\n",
			path,
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
		)
	}
	return nil
}

// Hover prints hover information.
func (f *Formatter) Hover(hover *lsp.Hover) error {
	if hover == nil {
		return nil
	}
	if f.JSON {
		return f.writeJSON(hover)
	}

	text := hover.HoverContents()
	// Strip markdown code fences for terminal display
	text = stripCodeFences(text)
	fmt.Fprintln(f.Writer, text)
	return nil
}

// DocumentSymbols prints hierarchical document symbols.
func (f *Formatter) DocumentSymbols(symbols []lsp.DocumentSymbol) error {
	if f.JSON {
		return f.writeJSON(symbols)
	}
	for _, sym := range symbols {
		printDocSymbol(f.Writer, sym, 0)
	}
	return nil
}

// SymbolInformations prints flat symbol information.
func (f *Formatter) SymbolInformations(symbols []lsp.SymbolInformation) error {
	if f.JSON {
		return f.writeJSON(symbols)
	}
	for _, sym := range symbols {
		path := lsp.URIToPath(sym.Location.URI)
		fmt.Fprintf(f.Writer, "%s:%d:%d %s %s\n",
			path,
			sym.Location.Range.Start.Line+1,
			sym.Location.Range.Start.Character+1,
			sym.Kind,
			sym.Name,
		)
	}
	return nil
}

// Diagnostics prints diagnostics.
func (f *Formatter) Diagnostics(uri string, diags []lsp.Diagnostic) error {
	if f.JSON {
		return f.writeJSON(map[string]interface{}{
			"uri":         uri,
			"diagnostics": diags,
		})
	}
	path := lsp.URIToPath(uri)
	for _, d := range diags {
		fmt.Fprintf(f.Writer, "%s:%d:%d: %s: %s\n",
			path,
			d.Range.Start.Line+1,
			d.Range.Start.Character+1,
			d.Severity,
			d.Message,
		)
	}
	return nil
}

// AllDiagnostics prints diagnostics for multiple URIs.
func (f *Formatter) AllDiagnostics(allDiags map[string][]lsp.Diagnostic) error {
	if f.JSON {
		return f.writeJSON(allDiags)
	}
	for uri, diags := range allDiags {
		f.Diagnostics(uri, diags)
	}
	return nil
}

func (f *Formatter) writeJSON(v interface{}) error {
	enc := json.NewEncoder(f.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printDocSymbol(w io.Writer, sym lsp.DocumentSymbol, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(w, "%s%s %s (line %d)\n",
		indent,
		sym.Kind,
		sym.Name,
		sym.Range.Start.Line+1,
	)
	for _, child := range sym.Children {
		printDocSymbol(w, child, depth+1)
	}
}

func stripCodeFences(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
