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
	if len(refsAtDefinition) != 2 {
		t.Fatalf("ReferencesAt(definition) = %d, want 2", len(refsAtDefinition))
	}
	if refsAtDefinition[0].Field != "import" || refsAtDefinition[0].Location.URI != "/repo/eirctl.yaml" {
		t.Fatalf("ReferencesAt(definition)[0] = %+v, want import reference in /repo/eirctl.yaml", refsAtDefinition[0])
	}
	if refsAtDefinition[1].Location.URI != "/repo/eirctl.yaml" {
		t.Fatalf("ReferencesAt(definition)[1] uri = %q, want /repo/eirctl.yaml", refsAtDefinition[1].Location.URI)
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

func TestAnalyzeWorkspaceReportsUnresolvedReferenceAndOffersExactOptions(t *testing.T) {
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
		t.Fatal("missing test reference for unresolved lookup")
	}

	definitions := result.DefinitionsAt(reference.Location.URI, reference.Location.Range.Start)
	if len(definitions) != 0 {
		t.Fatalf("DefinitionsAt(unresolved reference) = %d, want 0", len(definitions))
	}

	completions := result.CompletionsAt(reference.Location.URI, reference.Location.Range.Start)
	if len(completions) != 2 {
		t.Fatalf("CompletionsAt(unresolved reference) = %d, want 2", len(completions))
	}
	if completions[0].Label != "bold" || completions[1].Label != "build" {
		t.Fatalf("completion labels = [%q, %q], want [bold, build]", completions[0].Label, completions[1].Label)
	}

	hasUnresolvedDiagnostic := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "unresolved-reference" && diagnostic.Range == reference.Location.Range {
			hasUnresolvedDiagnostic = true
			break
		}
	}
	if !hasUnresolvedDiagnostic {
		t.Fatal("missing unresolved-reference diagnostic for bld")
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

func TestAnalyzeWorkspaceCompletesDependsOnFromParentPipelineStages(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`tasks:
  build:
    command: echo build
  test:
    command: echo test
  publish:
    command: echo publish
pipelines:
  ci:
    - task: build
    - task: test
      depends_on: [build]
    - task: publish
      depends_on:
        - tst
  release:
    - task: publish
    - task: build
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{HomeDir: "/home/tester"})

	var buildRef analyze.Reference
	var testRef analyze.Reference
	for _, candidate := range result.References {
		if candidate.Field != "pipelines.depends_on" {
			continue
		}
		switch candidate.Name {
		case "build":
			buildRef = candidate
		case "tst":
			testRef = candidate
		}
	}

	if buildRef.Name == "" {
		t.Fatal("missing depends_on reference for build")
	}
	if testRef.Name == "" {
		t.Fatal("missing depends_on reference for tst")
	}

	buildDefinitions := result.DefinitionsAt(buildRef.Location.URI, buildRef.Location.Range.Start)
	if len(buildDefinitions) != 1 {
		t.Fatalf("DefinitionsAt(build depends_on) = %d, want 1", len(buildDefinitions))
	}
	if buildDefinitions[0].Kind != protocol.SymbolKindStage {
		t.Fatalf("build depends_on definition kind = %q, want stage", buildDefinitions[0].Kind)
	}
	if buildDefinitions[0].Name != "build" {
		t.Fatalf("build depends_on definition name = %q, want build", buildDefinitions[0].Name)
	}

	testCompletions := result.CompletionsAt(testRef.Location.URI, testRef.Location.Range.Start)
	if len(testCompletions) != 3 {
		t.Fatalf("CompletionsAt(test depends_on) = %d, want 3", len(testCompletions))
	}
	if testCompletions[0].Label != "build" || testCompletions[1].Label != "publish" || testCompletions[2].Label != "test" {
		t.Fatalf("depends_on completion labels = [%q, %q, %q], want [build, publish, test]", testCompletions[0].Label, testCompletions[1].Label, testCompletions[2].Label)
	}
	if testCompletions[0].Kind != protocol.SymbolKindStage {
		t.Fatalf("depends_on completion kind = %q, want stage", testCompletions[0].Kind)
	}
	for _, completion := range testCompletions {
		if completion.Source.Label != "/repo/eirctl.yaml" {
			t.Fatalf("depends_on completion source = %q, want /repo/eirctl.yaml", completion.Source.Label)
		}
	}

	hasUnresolvedDiagnostic := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "unresolved-reference" && diagnostic.Range == testRef.Location.Range {
			if diagnostic.Message != `unknown pipeline stage "tst" in depends_on for pipeline "ci"` {
				t.Fatalf("depends_on diagnostic message = %q", diagnostic.Message)
			}
			hasUnresolvedDiagnostic = true
			break
		}
	}
	if !hasUnresolvedDiagnostic {
		t.Fatal("missing unresolved-reference diagnostic for depends_on tst")
	}

	refsAtBuildStage := result.ReferencesAt(buildDefinitions[0].Location.URI, buildDefinitions[0].Location.Range.Start)
	if len(refsAtBuildStage) != 1 {
		t.Fatalf("ReferencesAt(build stage) = %d, want 1", len(refsAtBuildStage))
	}
	if refsAtBuildStage[0].Name != "build" {
		t.Fatalf("ReferencesAt(build stage) name = %q, want build", refsAtBuildStage[0].Name)
	}
}

func TestAnalyzeWorkspaceReportsSpecificUnknownReferenceMessages(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`tasks:
  build:
    context: missing-context
    command: echo build
pipelines:
  ci:
    - task: missing-task
    - pipeline: missing-pipeline
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{HomeDir: "/home/tester"})

	messages := map[string]bool{}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "unresolved-reference" {
			messages[diagnostic.Message] = true
		}
	}

	if !messages[`unknown context "missing-context" in tasks.context`] {
		t.Fatal("missing specific context diagnostic")
	}
	if !messages[`unknown task "missing-task" in pipelines.task`] {
		t.Fatal("missing specific task diagnostic")
	}
	if !messages[`unknown pipeline "missing-pipeline" in pipelines.pipeline`] {
		t.Fatal("missing specific pipeline diagnostic")
	}
}

func TestAnalyzeWorkspaceShowsTaskAndPipelineCompletionsByKeyword(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`tasks:
  build:
    command: echo build
  test:
    command: echo test
pipelines:
  deploy:
    - task: build
    - pipeline: deploy
  release:
    - task: test
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := analyze.AnalyzeWorkspace(root, analyze.Options{HomeDir: "/home/tester"})

	var taskRef analyze.Reference
	var pipelineRef analyze.Reference
	for _, candidate := range result.References {
		switch {
		case candidate.Field == "pipelines.task" && candidate.Name == "build":
			taskRef = candidate
		case candidate.Field == "pipelines.pipeline" && candidate.Name == "deploy":
			pipelineRef = candidate
		}
	}

	if taskRef.Name == "" {
		t.Fatal("missing task reference")
	}
	if pipelineRef.Name == "" {
		t.Fatal("missing pipeline reference")
	}

	taskCompletions := result.CompletionsAt(taskRef.Location.URI, taskRef.Location.Range.Start)
	if len(taskCompletions) != 2 {
		t.Fatalf("CompletionsAt(task) = %d, want 2", len(taskCompletions))
	}
	if taskCompletions[0].Label != "build" || taskCompletions[1].Label != "test" {
		t.Fatalf("task completion labels = [%q, %q], want [build, test]", taskCompletions[0].Label, taskCompletions[1].Label)
	}
	for _, completion := range taskCompletions {
		if completion.Kind != protocol.SymbolKindTask {
			t.Fatalf("task completion kind = %q, want task", completion.Kind)
		}
	}

	pipelineCompletions := result.CompletionsAt(pipelineRef.Location.URI, pipelineRef.Location.Range.Start)
	if len(pipelineCompletions) != 2 {
		t.Fatalf("CompletionsAt(pipeline) = %d, want 2", len(pipelineCompletions))
	}
	if pipelineCompletions[0].Label != "deploy" || pipelineCompletions[1].Label != "release" {
		t.Fatalf("pipeline completion labels = [%q, %q], want [deploy, release]", pipelineCompletions[0].Label, pipelineCompletions[1].Label)
	}
	for _, completion := range pipelineCompletions {
		if completion.Kind != protocol.SymbolKindPipeline {
			t.Fatalf("pipeline completion kind = %q, want pipeline", completion.Kind)
		}
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
