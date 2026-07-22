package main

import (
	"context"
	"crypto/sha1" //nolint:gosec // Tests construct Git object identities.
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const testRepository = "example/repository"

func TestValidateTreeAcceptsOnlyBoundedNonExecutableGoBlobs(t *testing.T) {
	goSize := int64(12)
	executableSize := int64(8)
	tree := treeResponse{Tree: []treeEntry{
		{Path: "cmd", Mode: "040000", Type: "tree", SHA: testSHA("tree")},
		{Path: "cmd/main.go", Mode: "100644", Type: "blob", SHA: testSHA("go"), Size: &goSize},
		{Path: "cmd/tool.go", Mode: "100755", Type: "blob", SHA: testSHA("executable"), Size: &executableSize},
		{Path: "go.mod", Mode: "100644", Type: "blob", SHA: testSHA("module"), Size: &executableSize},
	}}
	selected, total, err := validateTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Path != "cmd/main.go" || total != goSize {
		t.Fatalf("selected = %#v, total = %d", selected, total)
	}
}

func TestValidateTreeRejectsHostileEntries(t *testing.T) {
	one := int64(1)
	valid := func() treeResponse {
		return treeResponse{Tree: []treeEntry{{Path: "main.go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}
	}
	manyEntries := valid()
	for index := 1; index < maxTreeEntries+1; index++ {
		manyEntries.Tree = append(manyEntries.Tree, treeEntry{Path: "dir-" + decimal(index), Mode: "040000", Type: "tree", SHA: testSHA(decimal(index))})
	}
	manyGoFiles := treeResponse{}
	for index := 0; index < maxGoFiles+1; index++ {
		manyGoFiles.Tree = append(manyGoFiles.Tree, treeEntry{Path: "file-" + decimal(index) + ".go", Mode: "100644", Type: "blob", SHA: testSHA(decimal(index)), Size: &one})
	}
	large := int64(maxFileBytes + 1)
	aggregate := treeResponse{}
	chunk := int64(maxFileBytes)
	for index := 0; index < maxTotalBytes/maxFileBytes+1; index++ {
		aggregate.Tree = append(aggregate.Tree, treeEntry{Path: "aggregate-" + decimal(index) + ".go", Mode: "100644", Type: "blob", SHA: testSHA("aggregate-" + decimal(index)), Size: &chunk})
	}

	cases := []struct {
		name string
		tree treeResponse
	}{
		{name: "truncated tree", tree: treeResponse{Truncated: true, Tree: valid().Tree}},
		{name: "empty tree", tree: treeResponse{}},
		{name: "too many tree entries", tree: manyEntries},
		{name: "symlink", tree: treeResponse{Tree: []treeEntry{{Path: "link", Mode: "120000", Type: "blob", SHA: testSHA("link"), Size: &one}}}},
		{name: "submodule", tree: treeResponse{Tree: []treeEntry{{Path: "module", Mode: "160000", Type: "commit", SHA: testSHA("module")}}}},
		{name: "unknown mode", tree: treeResponse{Tree: []treeEntry{{Path: "main.go", Mode: "100600", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "unknown type", tree: treeResponse{Tree: []treeEntry{{Path: "main.go", Mode: "100644", Type: "tag", SHA: testSHA("main"), Size: &one}}}},
		{name: "invalid identity", tree: treeResponse{Tree: []treeEntry{{Path: "main.go", Mode: "100644", Type: "blob", SHA: "short", Size: &one}}}},
		{name: "absolute path", tree: treeResponse{Tree: []treeEntry{{Path: "/main.go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "traversal path", tree: treeResponse{Tree: []treeEntry{{Path: "a/../main.go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "dot path", tree: treeResponse{Tree: []treeEntry{{Path: "./main.go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "backslash path", tree: treeResponse{Tree: []treeEntry{{Path: `dir\main.go`, Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "control character path", tree: treeResponse{Tree: []treeEntry{{Path: "dir/ma\nin.go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "excessive path", tree: treeResponse{Tree: []treeEntry{{Path: strings.Repeat("a", maxPathBytes-2) + ".go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &one}}}},
		{name: "duplicate normalized path", tree: treeResponse{Tree: []treeEntry{
			{Path: "main.go", Mode: "100644", Type: "blob", SHA: testSHA("one"), Size: &one},
			{Path: "main.go", Mode: "100644", Type: "blob", SHA: testSHA("two"), Size: &one},
		}}},
		{name: "missing blob size", tree: treeResponse{Tree: []treeEntry{{Path: "main.go", Mode: "100644", Type: "blob", SHA: testSHA("main")}}}},
		{name: "excessive blob size", tree: treeResponse{Tree: []treeEntry{{Path: "main.go", Mode: "100644", Type: "blob", SHA: testSHA("main"), Size: &large}}}},
		{name: "too many Go files", tree: manyGoFiles},
		{name: "excessive aggregate bytes", tree: aggregate},
		{name: "no eligible Go source", tree: treeResponse{Tree: []treeEntry{{Path: "README.md", Mode: "100644", Type: "blob", SHA: testSHA("readme"), Size: &one}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := validateTree(tc.tree); err == nil {
				t.Fatal("validateTree() unexpectedly accepted hostile tree")
			}
		})
	}
}

func TestFetchBlobVerifiesResponseAndGitIdentity(t *testing.T) {
	data := []byte("package main\n")
	sha := gitBlobSHA(data)
	size := int64(len(data))
	entry := treeEntry{Path: "main.go", Mode: "100644", Type: "blob", SHA: sha, Size: &size}

	cases := []struct {
		name     string
		status   int
		response blobResponse
		wantErr  bool
	}{
		{name: "valid", status: http.StatusOK, response: blobResponse{SHA: sha, Size: size, Encoding: "base64", Content: base64.StdEncoding.EncodeToString(data)}},
		{name: "missing blob", status: http.StatusNotFound, wantErr: true},
		{name: "wrong response identity", status: http.StatusOK, response: blobResponse{SHA: testSHA("wrong"), Size: size, Encoding: "base64", Content: base64.StdEncoding.EncodeToString(data)}, wantErr: true},
		{name: "wrong response size", status: http.StatusOK, response: blobResponse{SHA: sha, Size: size + 1, Encoding: "base64", Content: base64.StdEncoding.EncodeToString(data)}, wantErr: true},
		{name: "wrong encoding", status: http.StatusOK, response: blobResponse{SHA: sha, Size: size, Encoding: "utf-8", Content: string(data)}, wantErr: true},
		{name: "invalid base64", status: http.StatusOK, response: blobResponse{SHA: sha, Size: size, Encoding: "base64", Content: "%%%"}, wantErr: true},
		{name: "wrong decoded identity", status: http.StatusOK, response: blobResponse{SHA: sha, Size: size, Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte("wrong bytes!!"))}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				if tc.status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tc.response)
				}
			}))
			defer server.Close()
			client := testClient(t, server)
			_, err := fetchBlob(context.Background(), client, testRepository, entry)
			if (err != nil) != tc.wantErr {
				t.Fatalf("fetchBlob() error = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}

func TestMaterializeWritesExclusiveFilesAndRechecksCurrentHead(t *testing.T) {
	data := []byte("package source\n")
	sha := gitBlobSHA(data)
	treeSHA := testSHA("tree")
	headSHA := testSHA("head")
	size := int64(len(data))
	server := githubServer(t, headSHA, treeSHA, treeEntry{Path: "pkg/source.go", Mode: "100644", Type: "blob", SHA: sha, Size: &size}, data, headSHA)
	defer server.Close()

	parent := t.TempDir()
	cfg := testConfig(server.URL, headSHA, filepath.Join(parent, "source"))
	client := testClient(t, server)
	result, err := materialize(context.Background(), client, osFileSystem{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.TreeSHA != treeSHA || result.FileCount != 1 || result.TotalSize != size {
		t.Fatalf("result = %#v", result)
	}
	contents, err := os.ReadFile(filepath.Join(cfg.Output, "pkg", "source.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(data) {
		t.Fatalf("contents = %q", contents)
	}
	info, err := os.Stat(filepath.Join(cfg.Output, "pkg", "source.go"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestMaterializeRejectsExistingDestinationAndSupersededHead(t *testing.T) {
	data := []byte("package source\n")
	sha := gitBlobSHA(data)
	treeSHA := testSHA("tree")
	headSHA := testSHA("head")
	size := int64(len(data))
	entry := treeEntry{Path: "source.go", Mode: "100644", Type: "blob", SHA: sha, Size: &size}

	t.Run("existing destination root", func(t *testing.T) {
		server := githubServer(t, headSHA, treeSHA, entry, data, headSHA)
		defer server.Close()
		output := filepath.Join(t.TempDir(), "source")
		if err := os.Mkdir(output, 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := testConfig(server.URL, headSHA, output)
		if _, err := materialize(context.Background(), testClient(t, server), osFileSystem{}, cfg); err == nil {
			t.Fatal("materialize() accepted an existing destination")
		}
	})

	t.Run("superseded pull request head", func(t *testing.T) {
		server := githubServer(t, headSHA, treeSHA, entry, data, testSHA("new-head"))
		defer server.Close()
		output := filepath.Join(t.TempDir(), "source")
		cfg := testConfig(server.URL, headSHA, output)
		if _, err := materialize(context.Background(), testClient(t, server), osFileSystem{}, cfg); err == nil {
			t.Fatal("materialize() accepted a superseded pull-request head")
		}
		if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed materialization left source root behind: %v", err)
		}
	})
}

func TestWriteExclusiveRejectsExistingFilesAndShortWrites(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := writeExclusive(osFileSystem{}, root, "main.go", []byte("new")); err == nil {
			t.Fatal("writeExclusive() overwrote an existing file")
		}
	})

	t.Run("symlink parent", func(t *testing.T) {
		root := t.TempDir()
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(root, "pkg")); err != nil {
			t.Fatal(err)
		}
		if err := writeExclusive(osFileSystem{}, root, "pkg/main.go", []byte("new")); err == nil {
			t.Fatal("writeExclusive() followed a symlink parent")
		}
	})

	t.Run("short write", func(t *testing.T) {
		if err := writeOnce(shortWriter{}, []byte("source")); !errors.Is(err, errShortWrite) {
			t.Fatalf("writeOnce() error = %v", err)
		}
	})
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }

func githubServer(t *testing.T, headSHA, treeSHA string, entry treeEntry, data []byte, currentHead string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/git/commits/"+headSHA):
			_ = json.NewEncoder(w).Encode(commitResponse{SHA: headSHA, Tree: struct {
				SHA string `json:"sha"`
			}{SHA: treeSHA}})
		case strings.HasSuffix(r.URL.Path, "/git/trees/"+treeSHA) && r.URL.Query().Get("recursive") == "1":
			_ = json.NewEncoder(w).Encode(treeResponse{SHA: treeSHA, Tree: []treeEntry{entry}})
		case strings.HasSuffix(r.URL.Path, "/git/blobs/"+entry.SHA):
			_ = json.NewEncoder(w).Encode(blobResponse{SHA: entry.SHA, Size: int64(len(data)), Encoding: "base64", Content: base64.StdEncoding.EncodeToString(data)})
		case strings.HasSuffix(r.URL.Path, "/pulls/17"):
			var pull pullResponse
			pull.Number = 17
			pull.Base.Ref = "main"
			pull.Base.Repo.FullName = testRepository
			pull.Head.Repo.FullName = testRepository
			pull.Head.SHA = currentHead
			_ = json.NewEncoder(w).Encode(pull)
		default:
			http.NotFound(w, r)
		}
	}))
}

func testConfig(apiURL, headSHA, output string) config {
	return config{
		APIURL: apiURL, BaseRepository: testRepository, BaseBranch: "main",
		HeadRepository: testRepository, HeadSHA: headSHA, PullRequest: 17,
		Output: output, Bounds: reviewedBounds, Token: "test-token",
	}
}

func testClient(t *testing.T, server *httptest.Server) *apiClient {
	t.Helper()
	cfg := config{APIURL: server.URL, Token: "test-token"}
	client, err := newAPIClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	client.client = server.Client()
	return client
}

func gitBlobSHA(data []byte) string {
	digest := sha1.Sum(append([]byte("blob "+decimal(len(data))+"\x00"), data...)) //nolint:gosec // Git object identity.
	return hex.EncodeToString(digest[:])
}

func testSHA(seed string) string {
	digest := sha1.Sum([]byte(seed)) //nolint:gosec // Deterministic test identity only.
	return hex.EncodeToString(digest[:])
}

func decimal(value int) string { return strconv.Itoa(value) }
