package lsp_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/lang/analyze"
	"github.com/Ensono/eirctl/lang/lsp"
	langprotocol "github.com/Ensono/eirctl/lang/protocol"
)

func jsonRaw(value string) []byte { return []byte(value) }

type rpcTestResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Params json.RawMessage `json:"params"`
	Method string          `json:"method"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type rpcTestLocation struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
	} `json:"range"`
}

func rpcTestMessage(method string, id any, params any) map[string]any {
	message := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	if id != nil {
		message["id"] = id
	}
	return message
}

// runRPCMessages processes framed requests through the server's public API.
// A single buffered reader is reused so subsequent responses are not lost.
func runRPCMessages(
	t *testing.T,
	messages []map[string]any,
	opts ...lsp.ServerOpt,
) (*lsp.Server, []rpcTestResponse, error) {
	t.Helper()

	var input, output bytes.Buffer
	for _, message := range messages {
		payload, err := json.Marshal(message)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		input.WriteString(lsp.RPCMessageFormatter(len(payload), payload))
	}

	server, err := lsp.NewServer(&input, &output, opts...)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})

	serveErr := server.Serve()
	reader := bufio.NewReader(&output)
	var responses []rpcTestResponse

	for {
		payload, err := lsp.ReadMessage(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		var response rpcTestResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		responses = append(responses, response)
	}

	return server, responses, serveErr
}

func requireRPCSuccess(t *testing.T, response rpcTestResponse, id string) {
	t.Helper()

	if string(response.ID) != id {
		t.Fatalf("response ID = %s, want %s", response.ID, id)
	}
	if response.Error != nil {
		t.Fatalf("RPC error: %+v", response.Error)
	}
	if len(response.Result) == 0 {
		t.Fatal("response missing result property")
	}
}

func testFileURI(path string) string {
	return (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}).String()
}

func writeTestConfig(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestHoverMarkdownIncludesCandidates(t *testing.T) {
	value := lsp.HoverMarkdown(analyze.Hover{
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

func TestDidClosePublishesEmptyDiagnosticsArray(t *testing.T) {
	_, responses, err := runRPCMessages(t, []map[string]any{rpcTestMessage("textDocument/didClose", 1,
		map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/eirctl.yaml"},
		})},
	)
	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	if responses == nil {
		t.Fatal("expected responses not to be nil")
	}

	if responses[0].Method != "textDocument/publishDiagnostics" {
		t.Errorf("got = %q, want = %q", responses[0].Method, "textDocument/publishDiagnostics")
	}

	if !strings.Contains(string(responses[0].Params), `"diagnostics":[]`) {
		t.Errorf("want diagnostics array to be empty, got frame = %q", string(responses[0].Params))
	}

}

func TestRespondWithNilIncludesResultProperty(t *testing.T) {

	_, responses, err := runRPCMessages(t, []map[string]any{rpcTestMessage("textDocument/default", 1,
		map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/eirctl.yaml"},
		})},
	)
	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	if responses == nil {
		t.Fatal("expected responses not to be nil")
	}

	// not necessary to unmarshal from null to a Go nil type
	if responses == nil || string(responses[0].Result) != "null" {
		t.Fatalf("got %q, wanted nil", responses[0].Result)
	}
}

func TestDependsOnFallbackCompletionsStayWithinParentPipeline(t *testing.T) {
	// Arrange
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
	resp := []struct {
		InsertText string `json:"insertText"`
	}{}

	cfgPath := filepath.Join(t.TempDir(), "eirctl.yaml")

	writeTestConfig(t, cfgPath, string(content))

	// Act
	_, responses, err := runRPCMessages(t, []map[string]any{
		rpcTestMessage("textDocument/completion", 1,
			documentPositionTestParams(fmt.Sprintf("file://%s", cfgPath), 9, 11))})

	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	if responses == nil {
		t.Fatal("expected responses not to be nil")
	}
	// Assert
	_ = json.Unmarshal(responses[0].Result, &resp)
	if resp[0].InsertText != "build" {
		t.Fatalf("expected insertText = %q, got %q", "build", resp[0].InsertText)
	}
}

func initializeTestMessage(rootPath string) map[string]any {
	return rpcTestMessage("initialize", 1, map[string]any{
		"rootUri": testFileURI(rootPath),
	})
}

func documentPositionTestParams(uri string, line, character int) map[string]any {
	return map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position": map[string]int{
			"line":      line,
			"character": character,
		},
	}
}

func openTestDocumentMessage(uri string) map[string]any {
	return rpcTestMessage("textDocument/didOpen", nil, map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": "yaml",
			"version":    1,
			"text": "tasks:\n  build:\n    command: echo build\n" +
				"pipelines:\n  ci:\n    - task: build\n",
		},
	})
}

// assertImportedWorkspace checks definition resolution and root-file references
// using requests against the imported document rather than private analysis APIs.
func assertImportedWorkspace(t *testing.T, initializationRoot, importPath string) {
	t.Helper()

	rootPath := filepath.Join(initializationRoot, "eirctl.yaml")
	sharedPath := filepath.Join(initializationRoot, importPath, "shared.yaml")

	writeTestConfig(t, rootPath, fmt.Sprintf(`import:
  - ./%sshared.yaml
pipelines:
  ci:
    - task: test
`, importPath))
	writeTestConfig(t, sharedPath, `tasks:
  test:
    command: echo test
`)

	params := documentPositionTestParams(testFileURI(sharedPath), 1, 3)
	referenceParams := documentPositionTestParams(testFileURI(sharedPath), 1, 3)
	referenceParams["context"] = map[string]bool{"includeDeclaration": true}

	server, responses, err := runRPCMessages(t, []map[string]any{
		initializeTestMessage(initializationRoot),
		rpcTestMessage("textDocument/definition", 2, params),
		rpcTestMessage("textDocument/references", 3, referenceParams),
	})
	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if server.RootPath() != initializationRoot {
		t.Fatalf("RootPath() = %q, want %q", server.RootPath(), initializationRoot)
	}
	if len(responses) != 3 {
		t.Fatalf("response count = %d, want 3", len(responses))
	}

	requireRPCSuccess(t, responses[1], "2")
	var definitions []rpcTestLocation
	if err := json.Unmarshal(responses[1].Result, &definitions); err != nil {
		t.Fatalf("decode definitions: %v", err)
	}
	if len(definitions) != 1 || definitions[0].URI != testFileURI(sharedPath) {
		t.Fatalf("definitions = %+v, want one definition in %q", definitions, sharedPath)
	}

	requireRPCSuccess(t, responses[2], "3")
	var references []rpcTestLocation
	if err := json.Unmarshal(responses[2].Result, &references); err != nil {
		t.Fatalf("decode references: %v", err)
	}

	var hasImportSite, hasPipelineUsage bool
	for _, reference := range references {
		if reference.URI != testFileURI(rootPath) {
			continue
		}
		switch reference.Range.Start.Line {
		case 1:
			hasImportSite = true
		case 4:
			hasPipelineUsage = true
		}
	}
	if !hasImportSite {
		t.Fatal("missing import-site reference in root configuration")
	}
	if !hasPipelineUsage {
		t.Fatal("missing root pipeline usage reference for imported task")
	}
}

func TestAnalyzeAnchorsImportedFile(t *testing.T) {
	t.Run("ToWorkspaceRoot", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		assertImportedWorkspace(t, workspaceRoot, "")
	})
	t.Run("NestedWorkspaceConfigFromParentRoot", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		assertImportedWorkspace(t, workspaceRoot, t.Name()+"/")
	})
}

func TestResolveWorkspaceConfigPathPrefersProjectRootOverTestdata(t *testing.T) {
	t.Skip()
	parentRoot := t.TempDir()
	workspaceRoot := filepath.Join(parentRoot, "eirctl")
	projectConfig := filepath.Join(workspaceRoot, "eirctl.yaml")
	fixtureConfig := filepath.Join(workspaceRoot, "cmd", "testdata", "eirctl.yaml")
	currentPath := filepath.Join(workspaceRoot, "shared.yaml")

	writeTestConfig(t, projectConfig, `import:
  - ./shared.yaml
tasks:
  build:
    command: echo build
`)
	writeTestConfig(t, fixtureConfig, `tasks:
  fixture:
    command: echo fixture
`)
	writeTestConfig(t, currentPath, `pipelines:
  ci:
    - task: build
`)

	_, responses, err := runRPCMessages(t, []map[string]any{
		initializeTestMessage(parentRoot),
		rpcTestMessage("textDocument/definition", 2,
			documentPositionTestParams(testFileURI(currentPath), 2, 13)),
	})
	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	requireRPCSuccess(t, responses[1], "2")

	var definitions []rpcTestLocation
	if err := json.Unmarshal(responses[1].Result, &definitions); err != nil {
		t.Fatalf("decode definitions: %v", err)
	}
	if len(definitions) != 1 || definitions[0].URI != testFileURI(projectConfig) {
		t.Fatalf("definitions = %+v, want definition in %q", definitions, projectConfig)
	}
}

func TestServerDocumentRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		params func(string) map[string]any
	}{
		{
			name:   "documentSymbol",
			method: "textDocument/documentSymbol",
			params: func(uri string) map[string]any {
				return map[string]any{"textDocument": map[string]any{"uri": uri}}
			},
		},
		{
			name:   "completion",
			method: "textDocument/completion",
			params: func(uri string) map[string]any {
				return documentPositionTestParams(uri, 5, 12)
			},
		},
		{
			name:   "hover",
			method: "textDocument/hover",
			params: func(uri string) map[string]any {
				return documentPositionTestParams(uri, 5, 12)
			},
		},
		{
			name:   "references",
			method: "textDocument/references",
			params: func(uri string) map[string]any {
				params := documentPositionTestParams(uri, 5, 12)
				params["context"] = map[string]bool{"includeDeclaration": true}
				return params
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uri := testFileURI(filepath.Join(t.TempDir(), "eirctl.yaml"))
			_, responses, err := runRPCMessages(t, []map[string]any{
				openTestDocumentMessage(uri),
				rpcTestMessage(tc.method, 1, tc.params(uri)),
			})
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
			if len(responses) != 2 {
				t.Fatalf("response count = %d, want 2", len(responses))
			}
			requireRPCSuccess(t, responses[1], "1")
			if string(responses[1].Result) == "null" ||
				string(responses[1].Result) == "[]" {
				t.Fatalf("unexpected empty result = %s", responses[1].Result)
			}
		})

		t.Run(tc.name+"/invalid params", func(t *testing.T) {
			_, responses, err := runRPCMessages(t, []map[string]any{
				rpcTestMessage(tc.method, 1, map[string]any{}),
			})
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
			if len(responses) != 1 {
				t.Fatalf("response count = %d, want 1", len(responses))
			}
			response := responses[0]
			if string(response.ID) != "1" {
				t.Fatalf("response ID = %s, want 1", response.ID)
			}
			if response.Error == nil || response.Error.Code != lsp.ErrCodeInternal {
				t.Fatalf("RPC error = %+v, want code %d", response.Error, lsp.ErrCodeInternal)
			}
		})
	}
}

func TestServerDocumentNotifications(t *testing.T) {
	for _, method := range []string{
		"textDocument/didOpen",
		"textDocument/didChange",
	} {
		t.Run(method, func(t *testing.T) {
			uri := testFileURI(filepath.Join(t.TempDir(), "eirctl.yaml"))
			messages := []map[string]any{openTestDocumentMessage(uri)}

			if method == "textDocument/didChange" {
				messages = append(messages, rpcTestMessage(method, nil, map[string]any{
					"textDocument": map[string]any{"uri": uri, "version": 2},
					"contentChanges": []map[string]string{
						{"text": "tasks:\n  test:\n    command: echo test\n"},
					},
				}))
			}

			// Query symbols to verify the document state was updated.
			messages = append(messages, rpcTestMessage(
				"textDocument/documentSymbol", 1,
				map[string]any{"textDocument": map[string]any{"uri": uri}},
			))

			_, responses, err := runRPCMessages(t, messages)
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
			if len(responses) != len(messages) {
				t.Fatalf("response count = %d, want %d", len(responses), len(messages))
			}
			for _, response := range responses[:len(responses)-1] {
				if response.Method != "textDocument/publishDiagnostics" {
					t.Fatalf("unexpected notification = %q", response.Method)
				}
			}

			response := responses[len(responses)-1]
			requireRPCSuccess(t, response, "1")
			var symbols []struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(response.Result, &symbols); err != nil {
				t.Fatalf("decode symbols: %v", err)
			}

			wantName := "build"
			if method == "textDocument/didChange" {
				wantName = "test"
			}
			found := false
			for _, symbol := range symbols {
				if symbol.Name == wantName {
					found = true
				}
				if method == "textDocument/didChange" && symbol.Name == "build" {
					t.Fatal("document still contains the previous build task")
				}
			}
			if !found {
				t.Fatalf("symbols = %+v, missing %q", symbols, wantName)
			}
		})

		t.Run(method+"/invalid params", func(t *testing.T) {
			_, _, err := runRPCMessages(t, []map[string]any{
				rpcTestMessage(method, nil, map[string]any{}),
			})
			if err == nil {
				t.Fatal("Serve() returned nil for invalid notification params")
			}
		})
	}
}

func TestServerDidChangeErrors(t *testing.T) {
	t.Run("malformed document URI", func(t *testing.T) {
		_, _, err := runRPCMessages(t, []map[string]any{
			rpcTestMessage("textDocument/didChange", nil, map[string]any{
				"textDocument": map[string]any{
					"uri":     "http://[fe80::%31]/",
					"version": 2,
				},
				"contentChanges": []map[string]string{
					{"text": "tasks:\n  test:\n    command: echo test\n"},
				},
			}),
		})
		if err == nil {
			t.Fatal("Serve() returned nil for malformed document URI")
		}
	})
}

func TestServerHandleMessageDefault(t *testing.T) {
	_, responses, err := runRPCMessages(t, []map[string]any{
		rpcTestMessage("unknown/method", 99, map[string]any{}),
	})
	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("response count = %d, want 1", len(responses))
	}
	requireRPCSuccess(t, responses[0], "99")
	if string(responses[0].Result) != "null" {
		t.Fatalf("result = %s, want null", responses[0].Result)
	}
}

// TestDocumentURIs covers URI handling through document lifecycle notifications.
func TestDocumentURIs(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"unix URI", "file:///tmp/eirctl.yaml"},
		{"unix bare path", "/tmp/eirctl.yaml"},
		{"windows drive URI", "file:///c:/src/eirctl.yaml"},
		{"windows encoded colon", "file:///c%3A/src/eirctl.yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Opening and closing must resolve to the same document key.
			// After closing, documentSymbol must not see the in-memory task.
			_, responses, err := runRPCMessages(t, []map[string]any{
				openTestDocumentMessage(tc.uri),
				rpcTestMessage("textDocument/didClose", nil, map[string]any{
					"textDocument": map[string]any{"uri": tc.uri},
				}),
			})
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
			if len(responses) != 2 {
				t.Fatalf("response count = %d, want 2", len(responses))
			}

			response := responses[1]
			if response.Method != "textDocument/publishDiagnostics" {
				t.Fatalf("notification method = %q", response.Method)
			}
			var params struct {
				URI         string            `json:"uri"`
				Diagnostics []json.RawMessage `json:"diagnostics"`
			}
			if err := json.Unmarshal(response.Params, &params); err != nil {
				t.Fatalf("decode diagnostics: %v", err)
			}
			if params.URI != tc.uri {
				t.Fatalf("URI = %q, want %q", params.URI, tc.uri)
			}
			if params.Diagnostics == nil || len(params.Diagnostics) != 0 {
				t.Fatalf("diagnostics = %v, want empty array", params.Diagnostics)
			}
		})
	}
}

func TestUnsupportedDocumentURIsAreIgnored(t *testing.T) {
	for _, method := range []string{
		"textDocument/didOpen",
		"textDocument/didChange",
		"textDocument/didClose",
	} {
		t.Run(method, func(t *testing.T) {
			_, responses, err := runRPCMessages(t, []map[string]any{
				rpcTestMessage(method, nil, map[string]any{
					"textDocument": map[string]any{
						"uri": "https://example.invalid/eirctl.yaml",
					},
				}),
				rpcTestMessage("unknown/method", 1, map[string]any{}),
			})
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}
			if len(responses) != 1 {
				t.Fatalf("response count = %d, want 1", len(responses))
			}
			// A subsequent response proves the server continued processing.
			requireRPCSuccess(t, responses[0], "1")
		})
	}
}

func TestWorkspaceRootURIRoundTrip(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "workspace with spaces")

	server, responses, err := runRPCMessages(t, []map[string]any{
		initializeTestMessage(rootPath),
	})
	if err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("response count = %d, want 1", len(responses))
	}
	requireRPCSuccess(t, responses[0], "1")
	if server.RootPath() != filepath.Clean(rootPath) {
		t.Fatalf("RootPath() = %q, want %q", server.RootPath(), filepath.Clean(rootPath))
	}
}

func TestUriToPath(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{name: "unix path", uri: "file:///tmp/eirctl.yaml", want: filepath.FromSlash("/tmp/eirctl.yaml")},
		{name: "unix bare path", uri: "/tmp/eirctl.yaml", want: filepath.FromSlash("/tmp/eirctl.yaml")},
		{name: "windows drive URI", uri: "file:///c:/src/eirctl.yaml", want: filepath.FromSlash("c:/src/eirctl.yaml")},
		{name: "windows encoded colon", uri: "file:///c%3A/src/eirctl.yaml", want: filepath.FromSlash("c:/src/eirctl.yaml")},
		{name: "unsupported scheme", uri: "http://example.com", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lsp.UriToPath(tc.uri)
			if (err != nil) != tc.wantErr {
				t.Fatalf("uriToPath(%q) error = %v, wantErr %v", tc.uri, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("uriToPath(%q) = %q, want %q", tc.uri, got, tc.want)
			}
		})
	}
}

func TestPathToURI(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
		goos string
	}{
		{name: "unix path", path: "/tmp/eirctl.yaml", want: "file:///tmp/eirctl.yaml"},
		{name: "windows path", path: `c:\src\eirctl.yaml`, want: "file:///c:/src/eirctl.yaml", goos: "windows"},
		{name: "windows forward slash", path: "c:/src/eirctl.yaml", want: "file:///c:/src/eirctl.yaml", goos: "windows"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.goos != "" && runtime.GOOS != tc.goos {
				t.Skipf("test requires %s", tc.goos)
			}
			got := lsp.PathToURI(tc.path)
			if got != tc.want {
				t.Errorf("pathToURI(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestPathToURIRoundTrip(t *testing.T) {
	var path string
	if runtime.GOOS == "windows" {
		path = `c:\src\eirctl\eirctl.yaml`
	} else {
		path = "/tmp/eirctl.yaml"
	}
	uri := lsp.PathToURI(path)
	got, err := lsp.UriToPath(uri)
	if err != nil {
		t.Fatalf("uriToPath(pathToURI(%q)) error = %v", path, err)
	}
	if got != filepath.Clean(path) {
		t.Errorf("round-trip: got %q, want %q", got, filepath.Clean(path))
	}
}

// Testing all methods with a json unmarshal error
func Test_Server_Serve_handleMessageJsonUnmarshalError(t *testing.T) {
	ttests := map[string]struct {
		method  string
		id      any
		wantErr bool
	}{
		"initialize_err":                        {"initialize", "1", true},
		"initialize_err_false":                  {"initialize", nil, false},
		"textDocument/didOpen_err":              {"textDocument/didOpen", "1", true},
		"textDocument/didOpen_err_false":        {"textDocument/didOpen", nil, false},
		"textDocument/didChange_err":            {"textDocument/didChange", "1", true},
		"textDocument/didChange_err_false":      {"textDocument/didChange", nil, false},
		"textDocument/didClose_err":             {"textDocument/didClose", "1", true},
		"textDocument/didClose_err_false":       {"textDocument/didClose", nil, false},
		"textDocument/definition_err":           {"textDocument/definition", "1", true},
		"textDocument/definition_err_false":     {"textDocument/definition", nil, false},
		"textDocument/references_err":           {"textDocument/references", "1", true},
		"textDocument/references_err_false":     {"textDocument/references", nil, false},
		"textDocument/hover_err":                {"textDocument/hover", "1", true},
		"textDocument/hover_err_false":          {"textDocument/hover", nil, false},
		"textDocument/completion_err":           {"textDocument/completion", "1", true},
		"textDocument/completion_err_false":     {"textDocument/completion", nil, false},
		"textDocument/documentSymbol_err":       {"textDocument/documentSymbol", "1", true},
		"textDocument/documentSymbol_err_false": {"textDocument/documentSymbol", nil, false},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			_, responses, err := runRPCMessages(t, []map[string]any{
				rpcTestMessage(tt.method, tt.id, map[string]any{
					"textDocument": map[string]any{},
				}),
			}, lsp.WithJsonUnMarshalReqParams(func(data []byte, v any) error {
				return fmt.Errorf("unable to parse req Params")
			}))
			if err != nil {
				t.Fatalf("Serve() error = %v", err)
			}

			if tt.wantErr && responses[0].Error == nil {
				t.Errorf("expected JSON unmarshal error, got response = %v", responses[0])
			}
			if !tt.wantErr && (responses != nil && responses[0].Error != nil) {
				t.Errorf("expected no error, got response = %v", responses[0])
			}
			if tt.wantErr {
				if responses[0].Error == nil || responses[0].Error.Code != lsp.ErrCodeJSONUnmarshalReqParams {
					t.Fatalf("expected JSON unmarshal error, got response = %v", responses[0])
				}
			}
		})
	}
}

// // All supported methods tested over
// func TestServerHandleMessageDocumentSymbol(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":1,
//         "method":"textDocument/documentSymbol",
//         "params":{
//             "textDocument":{"uri":"file:///tmp/eirctl.yaml"}
//         }
//     }`))
// 	if err != nil {
// 		t.Fatalf("documentSymbol error = %v", err)
// 	}
// 	if !strings.Contains(out.String(), `"id":1`) {
// 		t.Fatalf("documentSymbol response = %q", out.String())
// 	}
// 	if !strings.Contains(out.String(), `"result"`) {
// 		t.Fatalf("documentSymbol response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageDocumentSymbolNegative(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":1,
//         "method":"textDocument/documentSymbol",
//         "params":{}
//     }`))
// 	if err != nil {
// 		t.Fatal("documentSymbol with invalid params returned non nil error")
// 	}
// 	if !strings.Contains(out.String(), fmt.Sprintf(`"code":%d`, lsp.ErrCodeInternal)) {
// 		t.Fatalf("documentSymbol response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageCompletion(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":2,
//         "method":"textDocument/completion",
//         "params":{
//             "textDocument":{"uri":"file:///tmp/eirctl.yaml"},
//             "position":{"line":5,"character":12}
//         }
//     }`))
// 	if err != nil {
// 		t.Fatalf("completion error = %v", err)
// 	}
// 	if !strings.Contains(out.String(), `"id":2`) ||
// 		!strings.Contains(out.String(), `"result"`) {
// 		t.Fatalf("completion response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageCompletionNegative(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":2,
//         "method":"textDocument/completion",
//         "params":{}
//     }`))
// 	if err != nil {
// 		t.Fatal("completion with invalid params returned non nil error")
// 	}
// 	if !strings.Contains(out.String(), fmt.Sprintf(`"code":%d`, ErrCodeInternal)) {
// 		t.Fatalf("completion response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageHover(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":3,
//         "method":"textDocument/hover",
//         "params":{
//             "textDocument":{"uri":"file:///tmp/eirctl.yaml"},
//             "position":{"line":5,"character":12}
//         }
//     }`))
// 	if err != nil {
// 		t.Fatalf("hover error = %v", err)
// 	}
// 	if !strings.Contains(out.String(), `"id":3`) ||
// 		!strings.Contains(out.String(), `"result"`) {
// 		t.Fatalf("hover response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageHoverNegative(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":3,
//         "method":"textDocument/hover",
//         "params":{}
//     }`))
// 	if err != nil {
// 		t.Fatal("hover with invalid params returned non nil error")
// 	}
// 	if !strings.Contains(out.String(), fmt.Sprintf(`"code":%d`, ErrCodeInternal)) {
// 		t.Fatalf("hover response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageReferences(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":4,
//         "method":"textDocument/references",
//         "params":{
//             "textDocument":{"uri":"file:///tmp/eirctl.yaml"},
//             "position":{"line":5,"character":12},
//             "context":{"includeDeclaration":true}
//         }
//     }`))
// 	if err != nil {
// 		t.Fatalf("references error = %v", err)
// 	}
// 	if !strings.Contains(out.String(), `"id":4`) ||
// 		!strings.Contains(out.String(), `"result"`) {
// 		t.Fatalf("references response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageReferencesNegative(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":4,
//         "method":"textDocument/references",
//         "params":{}
//     }`))
// 	if err != nil {
// 		t.Fatal("references with invalid params returned nil error")
// 	}
// 	if !strings.Contains(out.String(), fmt.Sprintf(`"code":%d`, ErrCodeInternal)) {
// 		t.Fatalf("references response = %q", out.String())
// 	}
// }

// func TestServerHandleMessageDidOpen(t *testing.T) {
// 	server, out := newHandlerTestServer(t)

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "method":"textDocument/didOpen",
//         "params":{
//             "textDocument":{
//                 "uri":"file:///tmp/eirctl.yaml",
//                 "languageId":"yaml",
//                 "version":1,
//                 "text":"tasks:\n  build:\n    command: echo build\n"
//             }
//         }
//     }`))
// 	if err != nil {
// 		t.Fatalf("didOpen error = %v", err)
// 	}
// 	if !strings.Contains(out.String(), `"method":"textDocument/publishDiagnostics"`) {
// 		t.Fatalf("didOpen output = %q", out.String())
// 	}
// }

// func TestServerHandleMessageDidOpenNegative(t *testing.T) {
// 	server, _ := newHandlerTestServer(t)

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "method":"textDocument/didOpen",
//         "params":{}
//     }`))
// 	if err == nil {
// 		t.Fatal("didOpen with invalid params returned nil error")
// 	}
// }

// func TestServerHandleMessageDidChange(t *testing.T) {
// 	server, out := newHandlerTestServer(t)
// 	openHandlerTestDocument(t, server)
// 	out.Reset()

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "method":"textDocument/didChange",
//         "params":{
//             "textDocument":{
//                 "uri":"file:///tmp/eirctl.yaml",
//                 "version":2
//             },
//             "contentChanges":[{
//                 "text":"tasks:\n  test:\n    command: echo test\n"
//             }]
//         }
//     }`))
// 	if err != nil {
// 		t.Fatalf("didChange error = %v", err)
// 	}
// 	if !strings.Contains(out.String(), `"method":"textDocument/publishDiagnostics"`) {
// 		t.Fatalf("didChange output = %q", out.String())
// 	}
// }

// func TestServerHandleMessageDidChangeNegative(t *testing.T) {

// 	t.Run("invalid params", func(t *testing.T) {
// 		server, _ := newHandlerTestServer(t)

// 		err := server.handleMessage([]byte(`{
// 			"jsonrpc":"2.0",
// 			"method":"textDocument/didChange",
// 			"params":{}
// 		}`))
// 		if err == nil {
// 			t.Fatal("didChange with invalid params returned nil error")
// 		}
// 	})
// 	t.Run("json messageParse err", func(t *testing.T) {
// 		server, _ := newHandlerTestServer(t, WithJsonUnMarshalReqParams(func(data []byte, v any) error {
// 			return fmt.Errorf("failed to parse req params")
// 		}))
// 		err := server.handleMessage([]byte(`{
// 			"jsonrpc":"2.0",
// 			"method":"textDocument/didChange",
// 			"params":{"textDocument":{"fooo":"","bar":2}}
// 		}`))
// 		if !errors.Is(err, ErrMessageJsonParse) {
// 			t.Fatal("didChange with invalid params returned nil error")
// 		}
// 	})
// 	t.Run("document uriParse err", func(t *testing.T) {
// 		server, _ := newHandlerTestServer(t, WithJsonUnMarshalReqParams(func(data []byte, v any) error {
// 			return json.Unmarshal(data, v)
// 		}))

// 		err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "method":"textDocument/didChange",
//         "params":{
//             "textDocument":{
//                 "uri":"http://[fe80::%31]/",
//                 "version":2
//             },
//             "contentChanges":[{
//                 "text":"tasks:\n  test:\n    command: echo test\n"
//             }]
//         }
// }`))
// 		if err == nil {
// 			t.Fatal("didChange with invalid params returned nil error")
// 		}
// 	})
// }

// func TestServerHandleMessageDefault(t *testing.T) {
// 	server, _ := newHandlerTestServer(t)

// 	err := server.handleMessage([]byte(`{
//         "jsonrpc":"2.0",
//         "id":99,
//         "method":"unknown/method",
//         "params":{}
//     }`))
// 	if err != nil {
// 		t.Fatal("unknown method returned non nil error")
// 	}
// }
