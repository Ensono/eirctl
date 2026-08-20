package lsp

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

func TestDidClosePublishesEmptyDiagnosticsArray(t *testing.T) {
	var out bytes.Buffer
	server, err := NewServer(strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	payload := []byte(`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":"file:///tmp/eirctl.yaml"}}}`)
	if err := server.handleMessage(payload); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	frame := out.String()
	if !strings.Contains(frame, `"method":"textDocument/publishDiagnostics"`) {
		t.Fatalf("frame = %q", frame)
	}
	if !strings.Contains(frame, `"diagnostics":[]`) {
		t.Fatalf("frame = %q", frame)
	}
}

func TestRespondWithNilIncludesResultProperty(t *testing.T) {
	var out bytes.Buffer
	server, err := NewServer(strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if err := server.respond(jsonRaw(`1`), nil); err != nil {
		t.Fatalf("respond() error = %v", err)
	}

	frame := out.String()
	if !strings.Contains(frame, `"result":null`) {
		t.Fatalf("frame = %q", frame)
	}
}

func TestDependsOnFallbackCompletionsStayWithinParentPipeline(t *testing.T) {
	server, err := NewServer(strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	content := []byte(`tasks:
  build:
    command: echo build
  publish:
    command: echo publish
pipelines:
  ci:
    - task: build
      depends_on:
        -
  release:
    - task: publish
`)
	result := analyze.Result{
		StageSymbols: []langprotocol.Symbol{
			{Name: "build", Kind: langprotocol.SymbolKindStage, Scope: "/repo/eirctl.yaml\x00ci", Source: langprotocol.DocumentSource{Label: "/repo/eirctl.yaml"}},
			{Name: "publish", Kind: langprotocol.SymbolKindStage, Scope: "/repo/eirctl.yaml\x00ci", Source: langprotocol.DocumentSource{Label: "/repo/eirctl.yaml"}},
			{Name: "publish", Kind: langprotocol.SymbolKindStage, Scope: "/repo/eirctl.yaml\x00release", Source: langprotocol.DocumentSource{Label: "/repo/eirctl.yaml"}},
		},
	}

	items := server.dependsOnFallbackCompletions("/repo/eirctl.yaml", content, lspPosition{Line: 9, Character: 10}, result)
	if len(items) != 2 {
		t.Fatalf("dependsOnFallbackCompletions() = %d, want 2", len(items))
	}
	if items[0].Label != "build" || items[1].Label != "publish" {
		t.Fatalf("dependsOnFallbackCompletions labels = [%q, %q], want [build, publish]", items[0].Label, items[1].Label)
	}
	for _, item := range items {
		if item.Source.Label != "/repo/eirctl.yaml" {
			t.Fatalf("completion source = %q, want /repo/eirctl.yaml", item.Source.Label)
		}
	}
}

func TestAnalyzeAnchorsImportedFileToWorkspaceRoot(t *testing.T) {
	workspaceRoot := t.TempDir()
	rootPath := filepath.Join(workspaceRoot, "eirctl.yaml")
	sharedPath := filepath.Join(workspaceRoot, "shared.yaml")

	rootContent := []byte(`import:
  - ./shared.yaml
pipelines:
  ci:
    - task: test
`)
	sharedContent := []byte(`tasks:
  test:
    command: echo test
`)

	if err := os.WriteFile(rootPath, rootContent, 0o644); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}
	if err := os.WriteFile(sharedPath, sharedContent, 0o644); err != nil {
		t.Fatalf("WriteFile(shared) error = %v", err)
	}

	server, err := NewServer(strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.rootPath = workspaceRoot

	result, currentPath, err := server.analyze(pathToURI(sharedPath))
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if currentPath != sharedPath {
		t.Fatalf("analyze() currentPath = %q, want %q", currentPath, sharedPath)
	}

	definitions := result.Definitions("test", langprotocol.SymbolKindTask)
	if len(definitions) != 1 {
		t.Fatalf("Definitions(test, task) = %d, want 1", len(definitions))
	}

	references := result.ReferencesAt(definitions[0].Location.URI, definitions[0].Location.Range.Start)
	if len(references) < 2 {
		t.Fatalf("ReferencesAt(imported definition) = %d, want at least 2", len(references))
	}

	hasImportSite := false
	hasUsageInRoot := false
	for _, reference := range references {
		if reference.Field == "import" && reference.Location.URI == rootPath {
			hasImportSite = true
		}
		if reference.Field == "pipelines.task" && reference.Location.URI == rootPath {
			hasUsageInRoot = true
		}
	}

	if !hasImportSite {
		t.Fatal("missing import-site reference in root eirctl.yaml")
	}
	if !hasUsageInRoot {
		t.Fatal("missing root pipeline usage reference for imported task")
	}
}

func TestAnalyzeDiscoversNestedWorkspaceConfigFromParentRoot(t *testing.T) {
	parentRoot := t.TempDir()
	workspaceRoot := filepath.Join(parentRoot, "eirctl")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
	}

	rootPath := filepath.Join(workspaceRoot, "eirctl.yaml")
	sharedPath := filepath.Join(workspaceRoot, "shared.yaml")

	rootContent := []byte(`import:
  - ./shared.yaml
pipelines:
  ci:
    - task: test
`)
	sharedContent := []byte(`tasks:
  test:
    command: echo test
`)

	if err := os.WriteFile(rootPath, rootContent, 0o644); err != nil {
		t.Fatalf("WriteFile(root) error = %v", err)
	}
	if err := os.WriteFile(sharedPath, sharedContent, 0o644); err != nil {
		t.Fatalf("WriteFile(shared) error = %v", err)
	}

	server, err := NewServer(strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.rootPath = parentRoot

	result, currentPath, err := server.analyze(pathToURI(sharedPath))
	if err != nil {
		t.Fatalf("analyze() error = %v", err)
	}
	if currentPath != sharedPath {
		t.Fatalf("analyze() currentPath = %q, want %q", currentPath, sharedPath)
	}

	definitions := result.Definitions("test", langprotocol.SymbolKindTask)
	if len(definitions) != 1 {
		t.Fatalf("Definitions(test, task) = %d, want 1", len(definitions))
	}

	references := result.ReferencesAt(definitions[0].Location.URI, definitions[0].Location.Range.Start)
	hasUsageInRoot := false
	for _, reference := range references {
		if reference.Field == "pipelines.task" && reference.Location.URI == rootPath {
			hasUsageInRoot = true
			break
		}
	}

	if !hasUsageInRoot {
		t.Fatal("missing root pipeline usage reference when rootPath points to parent folder")
	}
}

func TestResolveWorkspaceConfigPathPrefersProjectRootOverTestdata(t *testing.T) {
	parentRoot := t.TempDir()
	workspaceRoot := filepath.Join(parentRoot, "eirctl")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "cmd", "testdata"), 0o755); err != nil {
		t.Fatalf("MkdirAll(testdata) error = %v", err)
	}

	projectConfig := filepath.Join(workspaceRoot, "eirctl.yaml")
	testdataConfig := filepath.Join(workspaceRoot, "cmd", "testdata", "eirctl.yaml")

	if err := os.WriteFile(projectConfig, []byte("tasks:\n  build:\n    command: echo build\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(project eirctl.yaml) error = %v", err)
	}
	if err := os.WriteFile(testdataConfig, []byte("tasks:\n  fixture:\n    command: echo fixture\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(testdata eirctl.yaml) error = %v", err)
	}

	server, err := NewServer(strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.rootPath = parentRoot

	resolved := server.resolveWorkspaceConfigPath()
	if resolved != projectConfig {
		t.Fatalf("resolveWorkspaceConfigPath() = %q, want %q", resolved, projectConfig)
	}
}

func jsonRaw(value string) []byte { return []byte(value) }
