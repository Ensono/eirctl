package analyze_test

import (
	"errors"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/lang/analyze"
	"github.com/Ensono/eirctl/lang/ast"
	"github.com/Ensono/eirctl/lang/protocol"
	"github.com/Ensono/eirctl/lang/workspace"
)

func TestAnalyzeDocumentIndexesSymbolsReferencesAndImports(t *testing.T) {
	content := []byte(`import:
  - https://example.invalid/shared.yaml
  - ./local.yaml
  - src: git::https://github.com/Ensono/eirctl//shared/build/go/eirctl.yaml?ref=main
contexts:
  docker:
    dir: .
tasks:
  build:
    context: docker
    command: echo build
  broken:
    context: missing-context
    command: echo broken
pipelines:
  ci:
    - task: build
    - pipeline: deploy
watchers:
  file-watch:
    events: [write]
    watch: [.]
    task: missing-task
`)

	doc, err := ast.Parse("/repo/eirctl.yaml", content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeDocument(doc, analyze.Options{BaseDir: "/repo", HomeDir: "/home/tester"})

	if len(result.Symbols) != 5 {
		t.Fatalf("AnalyzeDocument() symbols = %d, want 5", len(result.Symbols))
	}
	if len(result.References) != 5 {
		t.Fatalf("AnalyzeDocument() references = %d, want 5", len(result.References))
	}
	if len(result.Imports) != 3 {
		t.Fatalf("AnalyzeDocument() imports = %d, want 3", len(result.Imports))
	}
	if len(result.Diagnostics) != 3 {
		t.Fatalf("AnalyzeDocument() diagnostics = %d, want 3", len(result.Diagnostics))
	}

	assertHasSymbol(t, result.Symbols, protocol.SymbolKindContext, "docker")
	assertHasSymbol(t, result.Symbols, protocol.SymbolKindTask, "build")
	assertHasSymbol(t, result.Symbols, protocol.SymbolKindTask, "broken")
	assertHasSymbol(t, result.Symbols, protocol.SymbolKindPipeline, "ci")
	assertHasSymbol(t, result.Symbols, protocol.SymbolKindWatcher, "file-watch")

	if result.Imports[0].Resolved.Path != config.GetCachePath("/home/tester", "https://example.invalid/shared.yaml") {
		t.Fatalf("first import path = %q", result.Imports[0].Resolved.Path)
	}
	if result.Imports[1].Resolved.Path != "/repo/local.yaml" {
		t.Fatalf("second import path = %q, want /repo/local.yaml", result.Imports[1].Resolved.Path)
	}
	if result.Imports[2].Resolved.Kind != workspace.ImportKindGit {
		t.Fatalf("third import kind = %q, want %q", result.Imports[2].Resolved.Kind, workspace.ImportKindGit)
	}
}

func TestAnalyzeDocumentReportsDuplicateDefinitions(t *testing.T) {
	content := []byte(`tasks:
  build:
    command: echo build
  build:
    command: echo duplicate
`)

	doc, err := ast.Parse("/repo/eirctl.yaml", content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeDocument(doc, analyze.Options{BaseDir: "/repo", HomeDir: "/home/tester"})
	if len(result.Diagnostics) != 1 {
		t.Fatalf("AnalyzeDocument() diagnostics = %d, want 1", len(result.Diagnostics))
	}
	if result.Diagnostics[0].URI != "/repo/eirctl.yaml" {
		t.Fatalf("diagnostic uri = %q, want /repo/eirctl.yaml", result.Diagnostics[0].URI)
	}
	if result.Diagnostics[0].Code != "duplicate-symbol" {
		t.Fatalf("diagnostic code = %q, want duplicate-symbol", result.Diagnostics[0].Code)
	}
	if len(result.Diagnostics[0].Related) != 1 {
		t.Fatalf("diagnostic related = %d, want 1", len(result.Diagnostics[0].Related))
	}
	if result.Diagnostics[0].Related[0].Location.URI != "/repo/eirctl.yaml" {
		t.Fatalf("related uri = %q", result.Diagnostics[0].Related[0].Location.URI)
	}
}

func TestAnalyzeWorkspaceResolvesReferencesFromImportedConfigs(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`import:
  - ./shared.yaml
tasks:
  build:
    context: docker
    command: echo build
pipelines:
  ci:
    - task: test
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{
		HomeDir: "/home/tester",
		ReadFile: func(path string) ([]byte, error) {
			if path != "/repo/shared.yaml" {
				return nil, errors.New("unexpected path")
			}
			return []byte(`contexts:
  docker:
    dir: .
tasks:
  test:
    command: echo test
`), nil
		},
	})

	var unresolved []protocol.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "unresolved-reference" || diagnostic.Code == "import-load-failed" {
			unresolved = append(unresolved, diagnostic)
		}
	}
	if len(unresolved) != 0 {
		t.Fatalf("AnalyzeWorkspace() diagnostics = %v, want no unresolved/import-load-failed diagnostics", unresolved)
	}

	testDefs := result.Definitions("test", protocol.SymbolKindTask)
	if len(testDefs) != 1 {
		t.Fatalf("Definitions(test, task) = %d, want 1", len(testDefs))
	}
	if testDefs[0].Source.Original != "./shared.yaml" {
		t.Fatalf("definition source original = %q, want ./shared.yaml", testDefs[0].Source.Original)
	}
	if testDefs[0].Source.Path != "/repo/shared.yaml" {
		t.Fatalf("definition source path = %q, want /repo/shared.yaml", testDefs[0].Source.Path)
	}
	if testDefs[0].Source.Label != "./shared.yaml -> /repo/shared.yaml" {
		t.Fatalf("definition source label = %q, want ./shared.yaml -> /repo/shared.yaml", testDefs[0].Source.Label)
	}

	refs := result.ReferencesFor("test", protocol.SymbolKindTask)
	if len(refs) != 1 {
		t.Fatalf("ReferencesFor(test, task) = %d, want 1", len(refs))
	}
	definition, ok := result.DefinitionFor(refs[0])
	if !ok {
		t.Fatal("DefinitionFor(reference) did not resolve imported symbol")
	}
	if definition.Source.Original != "./shared.yaml" {
		t.Fatalf("resolved definition source original = %q, want ./shared.yaml", definition.Source.Original)
	}

	defsAtReference := result.DefinitionsAt(refs[0].Location.URI, refs[0].Location.Range.Start)
	if len(defsAtReference) != 1 {
		t.Fatalf("DefinitionsAt(reference) = %d, want 1", len(defsAtReference))
	}
	if defsAtReference[0].Location.URI != "/repo/shared.yaml" {
		t.Fatalf("DefinitionsAt(reference) uri = %q, want /repo/shared.yaml", defsAtReference[0].Location.URI)
	}

	refsAtDefinition := result.ReferencesAt(testDefs[0].Location.URI, testDefs[0].Location.Range.Start)
	if len(refsAtDefinition) != 1 {
		t.Fatalf("ReferencesAt(definition) = %d, want 1", len(refsAtDefinition))
	}
	if refsAtDefinition[0].Location.URI != "/repo/eirctl.yaml" {
		t.Fatalf("ReferencesAt(definition) uri = %q, want /repo/eirctl.yaml", refsAtDefinition[0].Location.URI)
	}
}

func TestAnalyzeWorkspaceReportsDuplicateDefinitionsAcrossImports(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`import:
  - ./shared.yaml
tasks:
  build:
    command: echo root
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{
		HomeDir: "/home/tester",
		ReadFile: func(path string) ([]byte, error) {
			if path != "/repo/shared.yaml" {
				return nil, errors.New("unexpected path")
			}
			return []byte(`tasks:
  build:
    command: echo imported
`), nil
		},
	})

	var duplicate *protocol.Diagnostic
	for index := range result.Diagnostics {
		if result.Diagnostics[index].Code == "duplicate-symbol" {
			duplicate = &result.Diagnostics[index]
			break
		}
	}
	if duplicate == nil {
		t.Fatal("AnalyzeWorkspace() did not report duplicate-symbol across imports")
	}
	if len(duplicate.Related) != 1 {
		t.Fatalf("duplicate related = %d, want 1", len(duplicate.Related))
	}
	if duplicate.Related[0].Location.URI != "/repo/eirctl.yaml" {
		t.Fatalf("duplicate related uri = %q, want /repo/eirctl.yaml", duplicate.Related[0].Location.URI)
	}

	refs := result.ReferencesFor("build", protocol.SymbolKindTask)
	if len(refs) != 0 {
		t.Fatalf("ReferencesFor(build, task) = %d, want 0", len(refs))
	}

	defs := result.Definitions("build", protocol.SymbolKindTask)
	if len(defs) != 2 {
		t.Fatalf("Definitions(build, task) = %d, want 2", len(defs))
	}
}

func TestAnalyzeWorkspaceReportsImportLoadFailures(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`import:
  - ./missing.yaml
tasks:
  build:
    context: docker
    command: echo build
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{
		HomeDir: "/home/tester",
		ReadFile: func(path string) ([]byte, error) {
			return nil, errors.New("not found")
		},
	})

	hasImportLoadFailure := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "import-load-failed" {
			hasImportLoadFailure = true
			break
		}
	}
	if !hasImportLoadFailure {
		t.Fatal("AnalyzeWorkspace() did not report import-load-failed diagnostic")
	}
}

func TestAnalyzeWorkspaceReturnsFuzzyDefinitionCandidates(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`tasks:
  build:
    command: echo build
  bold:
    command: echo bold
pipelines:
  ci:
    - task: bld
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{HomeDir: "/home/tester"})

	var reference analyze.Reference
	ok := false
	for _, candidate := range result.References {
		if candidate.Name == "bld" && candidate.Kind == protocol.SymbolKindTask {
			reference = candidate
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("missing test reference for fuzzy lookup")
	}

	definitions := result.DefinitionsAt(reference.Location.URI, reference.Location.Range.Start)
	if len(definitions) != 2 {
		t.Fatalf("DefinitionsAt(fuzzy reference) = %d, want 2", len(definitions))
	}
	seen := map[string]bool{}
	for _, definition := range definitions {
		seen[definition.Name] = true
	}
	if !seen["build"] || !seen["bold"] {
		t.Fatalf("fuzzy definitions = %v, want build and bold", seen)
	}
}

func TestAnalyzeWorkspaceHoverAndCompletions(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`contexts:
  docker:
    dir: .
tasks:
  build:
    context: docker
    command: echo build
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{HomeDir: "/home/tester"})

	var reference analyze.Reference
	found := false
	for _, candidate := range result.References {
		if candidate.Field == "tasks.context" {
			reference = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing context reference")
	}

	hover, ok := result.HoverAt(reference.Location.URI, reference.Location.Range.Start)
	if !ok {
		t.Fatal("HoverAt() did not return hover for reference")
	}
	if len(hover.Matches) != 1 {
		t.Fatalf("hover matches = %d, want 1", len(hover.Matches))
	}
	if hover.Matches[0].Symbol.Name != "docker" {
		t.Fatalf("hover match = %q, want docker", hover.Matches[0].Symbol.Name)
	}

	completions := result.CompletionsAt(reference.Location.URI, reference.Location.Range.Start)
	if len(completions) != 1 {
		t.Fatalf("CompletionsAt() = %d, want 1", len(completions))
	}
	if completions[0].Label != "docker" {
		t.Fatalf("completion label = %q, want docker", completions[0].Label)
	}
	if completions[0].Match != analyze.MatchKindExact {
		t.Fatalf("completion match = %q, want exact", completions[0].Match)
	}
}

func assertHasSymbol(t *testing.T, symbols []protocol.Symbol, kind protocol.SymbolKind, name string) {
	t.Helper()
	for _, symbol := range symbols {
		if symbol.Kind == kind && symbol.Name == name {
			return
		}
	}
	t.Fatalf("missing symbol kind=%q name=%q", kind, name)
}
