package lsp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/lang/analyze"
	langprotocol "github.com/Ensono/eirctl/lang/protocol"
)

func TestHoverMarkdownIncludesCandidates(t *testing.T) {
	value := hoverMarkdown(analyze.Hover{
		Title:       "task reference build",
		Description: "Reference build in pipelines.task resolves to 2 candidate(s).",
		Source:      langprotocol.DocumentSource{Label: "./shared.yaml -> /repo/shared.yaml"},
		Matches: []analyze.DefinitionMatch{{
			Symbol: langprotocol.Symbol{Name: "build", Source: langprotocol.DocumentSource{Label: "/repo/eirctl.yaml"}},
			Match:  analyze.MatchKindExact,
		}},
	})
	if !strings.Contains(value, "Candidates:") {
		t.Fatalf("hover markdown = %q", value)
	}
	if !strings.Contains(value, "build") {
		t.Fatalf("hover markdown = %q", value)
	}
}

func TestWriteEmitsContentLengthFrame(t *testing.T) {
	var out bytes.Buffer
	server, err := NewServer(strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.respond(jsonRaw(`1`), map[string]any{"ok": true}); err != nil {
		t.Fatalf("respond() error = %v", err)
	}
	if !strings.Contains(out.String(), "Content-Length:") {
		t.Fatalf("frame = %q", out.String())
	}
}

func jsonRaw(value string) []byte { return []byte(value) }
