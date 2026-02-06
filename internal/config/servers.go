// Package config provides language server detection and configuration.
package config

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ServerConfig describes how to launch a language server.
type ServerConfig struct {
	Command []string
	Name    string
}

// Known language server configurations.
var knownServers = map[string][]ServerConfig{
	"go": {
		{Command: []string{"gopls", "serve"}, Name: "gopls"},
	},
	"python": {
		{Command: []string{"pylsp"}, Name: "pylsp"},
		{Command: []string{"pyright-langserver", "--stdio"}, Name: "pyright"},
	},
	"typescript": {
		{Command: []string{"typescript-language-server", "--stdio"}, Name: "typescript-language-server"},
	},
	"javascript": {
		{Command: []string{"typescript-language-server", "--stdio"}, Name: "typescript-language-server"},
	},
	"rust": {
		{Command: []string{"rust-analyzer"}, Name: "rust-analyzer"},
	},
	"c": {
		{Command: []string{"clangd"}, Name: "clangd"},
	},
	"cpp": {
		{Command: []string{"clangd"}, Name: "clangd"},
	},
	"java": {
		{Command: []string{"jdtls"}, Name: "jdtls"},
	},
	"ruby": {
		{Command: []string{"solargraph", "stdio"}, Name: "solargraph"},
	},
	"csharp": {
		{Command: []string{"OmniSharp", "--languageserver"}, Name: "omnisharp"},
	},
	"lua": {
		{Command: []string{"lua-language-server"}, Name: "lua-language-server"},
	},
}

// DetectServer finds an appropriate language server for the given file.
// Returns nil if no server is found.
func DetectServer(filePath string) (*ServerConfig, error) {
	lang := detectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("cannot detect language for %s", filePath)
	}

	configs, ok := knownServers[lang]
	if !ok {
		return nil, fmt.Errorf("no known language server for %s", lang)
	}

	for _, cfg := range configs {
		if _, err := exec.LookPath(cfg.Command[0]); err == nil {
			return &cfg, nil
		}
	}

	return nil, fmt.Errorf("no language server found on PATH for %s (tried: %s)",
		lang, serverNames(configs))
}

// ParseServerFlag parses a --server flag value into a command.
func ParseServerFlag(server string) []string {
	return strings.Fields(server)
}

func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx", ".h", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".cs":
		return "csharp"
	case ".lua":
		return "lua"
	default:
		return ""
	}
}

func serverNames(configs []ServerConfig) string {
	names := make([]string, len(configs))
	for i, c := range configs {
		names[i] = c.Name
	}
	return strings.Join(names, ", ")
}
