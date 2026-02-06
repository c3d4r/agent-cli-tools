// Package lsp provides LSP protocol types and client implementation.
// Uses only stdlib - no external dependencies.
package lsp

import "encoding/json"

// Position in a text document (0-indexed).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LocationLink represents a link between a source and a target location.
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem is an item to transfer a text document from client to server.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentPositionParams is a parameter for requests that operate on a position.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// DefinitionParams for textDocument/definition.
type DefinitionParams struct {
	TextDocumentPositionParams
}

// ReferenceContext controls whether declarations are included in references.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams for textDocument/references.
type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// HoverParams for textDocument/hover.
type HoverParams struct {
	TextDocumentPositionParams
}

// MarkupContent represents a string value with a specific markup kind.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover is the result of a hover request.
type Hover struct {
	Contents json.RawMessage `json:"contents"`
	Range    *Range          `json:"range,omitempty"`
}

// HoverContents extracts the hover text, handling both MarkupContent and string forms.
func (h *Hover) HoverContents() string {
	// Try MarkupContent first
	var mc MarkupContent
	if err := json.Unmarshal(h.Contents, &mc); err == nil && mc.Value != "" {
		return mc.Value
	}
	// Try plain string
	var s string
	if err := json.Unmarshal(h.Contents, &s); err == nil {
		return s
	}
	// Fallback: return raw
	return string(h.Contents)
}

// DocumentSymbolParams for textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SymbolKind represents the kind of a symbol.
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

var symbolKindNames = map[SymbolKind]string{
	SymbolKindFile:          "file",
	SymbolKindModule:        "module",
	SymbolKindNamespace:     "namespace",
	SymbolKindPackage:       "package",
	SymbolKindClass:         "class",
	SymbolKindMethod:        "method",
	SymbolKindProperty:      "property",
	SymbolKindField:         "field",
	SymbolKindConstructor:   "constructor",
	SymbolKindEnum:          "enum",
	SymbolKindInterface:     "interface",
	SymbolKindFunction:      "function",
	SymbolKindVariable:      "variable",
	SymbolKindConstant:      "constant",
	SymbolKindString:        "string",
	SymbolKindNumber:        "number",
	SymbolKindBoolean:       "boolean",
	SymbolKindArray:         "array",
	SymbolKindObject:        "object",
	SymbolKindKey:           "key",
	SymbolKindNull:          "null",
	SymbolKindEnumMember:    "enum_member",
	SymbolKindStruct:        "struct",
	SymbolKindEvent:         "event",
	SymbolKindOperator:      "operator",
	SymbolKindTypeParameter: "type_parameter",
}

func (sk SymbolKind) String() string {
	if name, ok := symbolKindNames[sk]; ok {
		return name
	}
	return "unknown"
}

// DocumentSymbol represents a symbol in a document (hierarchical).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation is the flat (non-hierarchical) variant.
type SymbolInformation struct {
	Name     string     `json:"name"`
	Kind     SymbolKind `json:"kind"`
	Location Location   `json:"location"`
}

// WorkspaceSymbolParams for workspace/symbol.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// ImplementationParams for textDocument/implementation.
type ImplementationParams struct {
	TextDocumentPositionParams
}

// DiagnosticSeverity represents the severity of a diagnostic.
type DiagnosticSeverity int

const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

func (ds DiagnosticSeverity) String() string {
	switch ds {
	case DiagnosticSeverityError:
		return "error"
	case DiagnosticSeverityWarning:
		return "warning"
	case DiagnosticSeverityInformation:
		return "info"
	case DiagnosticSeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Diagnostic represents a diagnostic (error, warning, etc.).
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// PublishDiagnosticsParams is sent from server to client.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// --- Initialize types ---

type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Definition      *DefinitionClientCapabilities      `json:"definition,omitempty"`
	References      *ReferencesClientCapabilities       `json:"references,omitempty"`
	Hover           *HoverClientCapabilities            `json:"hover,omitempty"`
	DocumentSymbol  *DocumentSymbolClientCapabilities   `json:"documentSymbol,omitempty"`
	Implementation  *ImplementationClientCapabilities   `json:"implementation,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCapabilities `json:"publishDiagnostics,omitempty"`
}

type DefinitionClientCapabilities struct {
	LinkSupport bool `json:"linkSupport,omitempty"`
}

type ReferencesClientCapabilities struct{}

type HoverClientCapabilities struct {
	ContentFormat []string `json:"contentFormat,omitempty"`
}

type DocumentSymbolClientCapabilities struct {
	HierarchicalDocumentSymbolSupport bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`
}

type ImplementationClientCapabilities struct {
	LinkSupport bool `json:"linkSupport,omitempty"`
}

type PublishDiagnosticsClientCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	DefinitionProvider     interface{} `json:"definitionProvider,omitempty"`
	ReferencesProvider     interface{} `json:"referencesProvider,omitempty"`
	HoverProvider          interface{} `json:"hoverProvider,omitempty"`
	DocumentSymbolProvider interface{} `json:"documentSymbolProvider,omitempty"`
	ImplementationProvider interface{} `json:"implementationProvider,omitempty"`
	WorkspaceSymbolProvider interface{} `json:"workspaceSymbolProvider,omitempty"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// DidOpenTextDocumentParams for textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidCloseTextDocumentParams for textDocument/didClose.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}
