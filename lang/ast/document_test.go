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
