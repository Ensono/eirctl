package workspace_test

import (
	"errors"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/lang/ast"
	"github.com/Ensono/eirctl/lang/workspace"
)

func TestLoadDocumentsRecursivelyLoadsImportedConfigs(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`import:
  - https://example.invalid/shared.yaml
tasks:
  build:
    context: docker
    command: echo build
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	sharedPath := config.GetCachePath("/home/tester", "https://example.invalid/shared.yaml")
	snapshot := workspace.LoadDocuments(root, workspace.LoadOptions{
		HomeDir: "/home/tester",
		ReadFile: func(path string) ([]byte, error) {
			if path != sharedPath {
				return nil, errors.New("unexpected path")
			}
			return []byte(`contexts:
  docker:
    dir: .
`), nil
		},
	})

	if len(snapshot.Documents) != 2 {
		t.Fatalf("LoadDocuments() documents = %d, want 2", len(snapshot.Documents))
	}
	if len(snapshot.Issues) != 0 {
		t.Fatalf("LoadDocuments() issues = %d, want 0", len(snapshot.Issues))
	}
	if snapshot.Documents[1].URI != sharedPath {
		t.Fatalf("imported uri = %q, want %q", snapshot.Documents[1].URI, sharedPath)
	}
	if snapshot.Sources[sharedPath].Original != "https://example.invalid/shared.yaml" {
		t.Fatalf("imported original = %q", snapshot.Sources[sharedPath].Original)
	}
	if snapshot.Sources[sharedPath].Label != "https://example.invalid/shared.yaml -> "+sharedPath {
		t.Fatalf("imported label = %q", snapshot.Sources[sharedPath].Label)
	}
	if !snapshot.Sources[sharedPath].FromCache {
		t.Fatal("imported source should be marked as cached")
	}
}

func TestLoadDocumentsSkipsFileImportsAndReportsReadFailures(t *testing.T) {
	root, err := ast.Parse("/repo/eirctl.yaml", []byte(`import:
  - src: https://example.invalid/config.yaml
    dest: local/config.yaml
  - ./missing.yaml
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	snapshot := workspace.LoadDocuments(root, workspace.LoadOptions{
		HomeDir: "/home/tester",
		ReadFile: func(path string) ([]byte, error) {
			return nil, errors.New("not found")
		},
	})

	if len(snapshot.Documents) != 1 {
		t.Fatalf("LoadDocuments() documents = %d, want 1", len(snapshot.Documents))
	}
	if len(snapshot.Issues) != 1 {
		t.Fatalf("LoadDocuments() issues = %d, want 1", len(snapshot.Issues))
	}
	if snapshot.Issues[0].Import.Raw != "./missing.yaml" {
		t.Fatalf("issue raw import = %q, want ./missing.yaml", snapshot.Issues[0].Import.Raw)
	}
}
