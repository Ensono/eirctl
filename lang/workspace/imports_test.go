package workspace_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/lang/workspace"
)

func TestResolveImport(t *testing.T) {
	absRaw := "/opt/eirctl/shared.yaml"
	absWant := filepath.FromSlash("/opt/eirctl/shared.yaml")
	if runtime.GOOS == "windows" {
		absRaw = `C:\opt\eirctl\shared.yaml`
		absWant = `C:\opt\eirctl\shared.yaml`
	}

	testCases := map[string]struct {
		baseDir string
		homeDir string
		raw     string
		kind    workspace.ImportKind
		path    string
		cached  bool
	}{
		"relative local import": {
			baseDir: "/repo",
			homeDir: "/home/tester",
			raw:     "shared/eirctl.yaml",
			kind:    workspace.ImportKindLocal,
			path:    filepath.FromSlash("/repo/shared/eirctl.yaml"),
		},
		"absolute local import": {
			baseDir: "/repo",
			homeDir: "/home/tester",
			raw:     absRaw,
			kind:    workspace.ImportKindLocal,
			path:    absWant,
		},
		"https import": {
			baseDir: "/repo",
			homeDir: "/home/tester",
			raw:     "https://example.invalid/shared.yaml",
			kind:    workspace.ImportKindURL,
			path:    config.GetCachePath("/home/tester", "https://example.invalid/shared.yaml"),
			cached:  true,
		},
		"git import": {
			baseDir: "/repo",
			homeDir: "/home/tester",
			raw:     "git::https://github.com/Ensono/eirctl//shared/build/go/eirctl.yaml?ref=main",
			kind:    workspace.ImportKindGit,
			path:    config.GetCachePath("/home/tester", "git::https://github.com/Ensono/eirctl//shared/build/go/eirctl.yaml?ref=main"),
			cached:  true,
		},
	}

	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			resolved := workspace.ResolveImport(tt.baseDir, tt.homeDir, tt.raw)
			if resolved.Kind != tt.kind {
				t.Fatalf("ResolveImport() kind = %q, want %q", resolved.Kind, tt.kind)
			}
			if resolved.Path != tt.path {
				t.Fatalf("ResolveImport() path = %q, want %q", resolved.Path, tt.path)
			}
			if resolved.FromCache != tt.cached {
				t.Fatalf("ResolveImport() cached = %t, want %t", resolved.FromCache, tt.cached)
			}
		})
	}
}
