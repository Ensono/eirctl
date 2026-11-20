package ast_test

import (
	"testing"

	"github.com/Ensono/eirctl/lang/ast"
)

func TestParseDocumentSections(t *testing.T) {
	content := []byte("tasks:\n  build:\n    command: echo build\ncontexts:\n  docker: {}\n")

	doc, err := ast.Parse("/repo/eirctl.yaml", content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	tasksKey, tasksValue, ok := doc.TopLevelSection("tasks")
	if !ok {
		t.Fatal("TopLevelSection(tasks) not found")
	}
	if tasksKey.Value != "tasks" {
		t.Fatalf("TopLevelSection(tasks) key = %q", tasksKey.Value)
	}

	entries := ast.MappingEntries(tasksValue)
	if len(entries) != 1 {
		t.Fatalf("MappingEntries(tasks) len = %d, want 1", len(entries))
	}
	if entries[0].Key.Value != "build" {
		t.Fatalf("task key = %q, want build", entries[0].Key.Value)
	}
}

func TestNodeRangeUsesZeroBasedCoordinates(t *testing.T) {
	content := []byte("tasks:\n  build:\n    command: echo build\n")

	doc, err := ast.Parse("/repo/eirctl.yaml", content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	_, tasksValue, ok := doc.TopLevelSection("tasks")
	if !ok {
		t.Fatal("TopLevelSection(tasks) not found")
	}

	entries := ast.MappingEntries(tasksValue)
	rng := doc.Location(entries[0].Key).Range
	if rng.Start.Line != 1 || rng.Start.Character != 2 {
		t.Fatalf("range start = %+v, want line=1 character=2", rng.Start)
	}
	if rng.End.Character <= rng.Start.Character {
		t.Fatalf("range end = %+v, want end after start", rng.End)
	}
}

func TestParseRecoveringSkipsMalformedNodeAndKeepsLaterSections(t *testing.T) {
	content := []byte(`tasks:
  build:
    command: echo build
  broken: build:
  publish:
    command: echo publish
pipelines:
  ci:
    - task: build
`)
	doc, diagnostics, err := ast.ParseRecovering("/repo/eirctl.yaml", content)
	if err != nil {
		t.Fatalf("ParseRecovering() error = %v", err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("ParseRecovering() diagnostics = 0, want at least 1")
	}

	_, tasksValue, ok := doc.TopLevelSection("tasks")
	if !ok {
		t.Fatal("TopLevelSection(tasks) not found")
	}
	entries := ast.MappingEntries(tasksValue)
	if len(entries) != 2 {
		t.Fatalf("MappingEntries(tasks) len = %d, want 2", len(entries))
	}
	if entries[0].Key.Value != "build" || entries[1].Key.Value != "publish" {
		t.Fatalf("task keys = %q, %q", entries[0].Key.Value, entries[1].Key.Value)
	}

	_, pipelinesValue, ok := doc.TopLevelSection("pipelines")
	if !ok {
		t.Fatal("TopLevelSection(pipelines) not found")
	}
	pipelines := ast.MappingEntries(pipelinesValue)
	if len(pipelines) != 1 || pipelines[0].Key.Value != "ci" {
		t.Fatalf("pipelines = %v", pipelines)
	}
}
