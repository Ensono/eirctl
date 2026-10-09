package lsp

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
	"strconv"
	"strings"

	"github.com/Ensono/eirctl/lang/analyze"
	"github.com/Ensono/eirctl/lang/ast"
	langprotocol "github.com/Ensono/eirctl/lang/protocol"
	"github.com/rs/zerolog"
)

const (
	ErrCodeInternal               = -32603
	ErrCodeJSONUnmarshalReqParams = -32602
)

var (
	RPCMessageFormatter = func(length int, payload []byte) string {
		return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", length, payload)
	}
	skipDirList = []string{".git", "node_modules"}
)

var (
	// ErrUnsupportedURIScheme is returned by uriToPath when the URI has a scheme
	// that the LSP server does not handle (e.g. "git", "untitled"). Callers
	// that receive this error from a notification handler should silently ignore
	// the request rather than terminating the server.
	ErrUnsupportedURIScheme = errors.New("unsupported URI scheme")
	ErrMessageJsonParse     = errors.New("json parse failure")
)

// Server defines the language server that implements the Language Server Protocol (LSP) for eirctl.
type Server struct {
	reader *bufio.Reader
	writer io.Writer

	config Config
	// homeDir is the user's home directory, used for resolving cache files referenced in imports.
	homeDir string
	// rootPath is the root path of the workspace, derived from the initialize request.
	//
	// all analysis is performed relative to this path.
	//
	// Currently, this is used to determine the entry point for analysis and to discover workspace configuration files.
	rootPath                    string
	docs                        map[string]string
	configPathDiscoveryComplete bool
	discoveredConfigPath        string
	closed                      bool
	// structured logger
	log zerolog.Logger
	// standin for json.Func
	jsonUnMarshalReq       func(data []byte, v any) error
	jsonUnMarshalReqParams func(data []byte, v any) error
}

type ServerOpt func(*Server)

func NewServer(in io.Reader, out io.Writer, opts ...ServerOpt) (*Server, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	s := &Server{
		reader:  bufio.NewReader(in),
		writer:  out,
		homeDir: homeDir,
		docs:    map[string]string{},
		// initializing the default logger to stderr with error level
		log:                    zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.ErrorLevel),
		config:                 Config{},
		jsonUnMarshalReq:       json.Unmarshal,
		jsonUnMarshalReqParams: json.Unmarshal,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func WithLogger(logger zerolog.Logger) ServerOpt {
	return func(s *Server) {
		s.log = logger
	}
}

func WithConfig(config Config) ServerOpt {
	return func(s *Server) {
		s.config = config
	}
}

func WithJsonUnMarshalReq(jsonUnMarshalReq func(data []byte, v any) error) ServerOpt {
	return func(s *Server) {
		s.jsonUnMarshalReq = jsonUnMarshalReq
	}
}

func WithJsonUnMarshalReqParams(jsonUnMarshalReqParams func(data []byte, v any) error) ServerOpt {
	return func(s *Server) {
		s.jsonUnMarshalReqParams = jsonUnMarshalReqParams
	}
}

func (s *Server) Serve() error {
	for !s.closed {
		payload, err := ReadMessage(s.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := s.handleMessage(payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Close() error {
	s.closed = true
	return nil
}

func (s *Server) RootPath() string {
	return s.rootPath
}

// handleMessage processes an incoming JSON-RPC message and dispatches it to the appropriate handler based on the method.
//
//nolint:gocognit,funlen,gocyclo,cyclop,maintidx
func (s *Server) handleMessage(payload []byte) error {
	var req requestEnvelope
	if err := s.jsonUnMarshalReq(payload, &req); err != nil {
		return err
	}

	switch req.Method {
	case "initialize":
		var params initializeParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		s.rootPath = deriveRootPath(params)
		s.configPathDiscoveryComplete = false
		s.discoveredConfigPath = ""
		s.log.Debug().Msgf("initialize rootPath=%q rootUri=%q workspaceFolders=%d", s.rootPath, params.RootURI, len(params.WorkspaceFolders))
		return s.respond(req.ID, map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync":       1,
				"definitionProvider":     true,
				"referencesProvider":     true,
				"hoverProvider":          true,
				"documentSymbolProvider": true,
				"completionProvider": map[string]any{
					"resolveProvider": false,
				},
			},
		})
	case "initialized":
		return nil
	case "shutdown":
		return s.respond(req.ID, nil)
	case "exit":
		s.closed = true
		return nil
	case "textDocument/didOpen":
		var params didOpenTextDocumentParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		path, err := UriToPath(params.TextDocument.URI)
		if err != nil {
			if errors.Is(err, ErrUnsupportedURIScheme) {
				s.log.Debug().Msgf("textDocument/didOpen: skipping unsupported URI %q", params.TextDocument.URI)
				return nil
			}
			return err
		}
		s.docs[path] = params.TextDocument.Text
		return s.publishDiagnostics(path)
	case "textDocument/didChange":
		var params didChangeTextDocumentParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		path, err := UriToPath(params.TextDocument.URI)
		if err != nil {
			if errors.Is(err, ErrUnsupportedURIScheme) {
				s.log.Debug().Msgf("textDocument/didChange: skipping unsupported URI %q", params.TextDocument.URI)
				return nil
			}
			return err
		}
		if len(params.ContentChanges) > 0 {
			s.docs[path] = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
		return s.publishDiagnostics(path)
	case "textDocument/didClose":
		var params didCloseTextDocumentParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		path, err := UriToPath(params.TextDocument.URI)
		if err != nil {
			if errors.Is(err, ErrUnsupportedURIScheme) {
				s.log.Debug().Msgf("textDocument/didClose: skipping unsupported URI %q", params.TextDocument.URI)
				return nil
			}
			return err
		}
		delete(s.docs, path)
		return s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: params.TextDocument.URI, Diagnostics: []lspDiagnostic{}})
	case "textDocument/definition":
		var params textDocumentPositionParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, ErrCodeInternal, err.Error())
		}
		defs := result.DefinitionsAt(path, toProtocolPosition(params.Position))
		return s.respond(req.ID, toLSPLocations(defs))
	case "textDocument/references":
		var params referenceParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, ErrCodeInternal, err.Error())
		}
		refs := result.ReferencesAt(path, toProtocolPosition(params.Position))
		locations := toLSPReferenceLocations(refs)
		if params.Context.IncludeDeclaration {
			for _, def := range result.DefinitionsAt(path, toProtocolPosition(params.Position)) {
				locations = append(locations, toLSPLocation(def.Location))
			}
		}
		return s.respond(req.ID, locations)
	case "textDocument/hover":
		var params textDocumentPositionParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, ErrCodeInternal, err.Error())
		}
		hover, ok := result.HoverAt(path, toProtocolPosition(params.Position))
		if !ok {
			return s.respond(req.ID, nil)
		}
		return s.respond(req.ID, lspHover{Contents: markupContent{Kind: "markdown", Value: HoverMarkdown(hover)}})
	case "textDocument/completion":
		var params completionParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, ErrCodeInternal, err.Error())
		}
		items := result.CompletionsAt(path, toProtocolPosition(params.Position))
		if len(items) == 0 {
			if content, readErr := s.readFile(path); readErr == nil {
				items = s.dependsOnFallbackCompletions(path, content, params.Position, result)
			}
		}
		return s.respond(req.ID, toLSPCompletionItems(items))
	case "textDocument/documentSymbol":
		var params documentSymbolParams
		if err := s.jsonUnMarshalReqParams(req.Params, &params); err != nil {
			return s.reqParamsError(&req, err)
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, ErrCodeInternal, err.Error())
		}
		return s.respond(req.ID, toLSPDocumentSymbols(result, path))
	default:
		if len(req.ID) > 0 {
			return s.respond(req.ID, nil)
		}
		return nil
	}
}

func (s *Server) reqParamsError(req *requestEnvelope, err error) error {
	if len(req.ID) == 0 {
		s.log.Debug().Err(fmt.Errorf("%w, %v", ErrMessageJsonParse, err)).Msg("invalid documentSymbol params in " + req.Method)
		return nil
	}
	return s.respondError(req.ID, ErrCodeJSONUnmarshalReqParams, err.Error())
}

func (s *Server) analyze(uri string) (analyze.Result, string, error) {
	path, err := UriToPath(uri)
	if err != nil {
		return analyze.Result{}, "", err
	}
	entryPath := s.analysisEntryPath(path)
	s.log.Debug().Msgf("analyze uri=%q currentPath=%q entryPath=%q rootPath=%q", uri, path, entryPath, s.rootPath)
	content, err := s.readFile(entryPath)
	if err != nil {
		s.log.Debug().Msgf("analyze read error entryPath=%q error=%v", entryPath, err)
		return analyze.Result{}, "", err
	}
	doc, parseDiagnostics, err := ast.ParseRecovering(entryPath, content)
	if err != nil && doc == nil {
		s.log.Debug().Msgf("analyze parse fatal entryPath=%q error=%v", entryPath, err)
		return analyze.Result{}, "", err
	}
	if err != nil {
		s.log.Debug().Msgf("analyze parse recovered entryPath=%q diagnostics=%d error=%v", entryPath, len(parseDiagnostics), err)
	}
	result := analyze.AnalyzeWorkspace(doc, analyze.Options{HomeDir: s.homeDir, ReadFile: s.readFile, BaseDir: filepath.Dir(entryPath)})
	result.Diagnostics = append(result.Diagnostics, parseDiagnostics...)
	s.log.Debug().Msgf("analyze complete symbols=%d diagnostics=%d", len(result.Symbols), len(result.Diagnostics))
	return result, path, nil
}

// analysisEntryPath determines the entry path for analysis based on the current path and server root configuration.
//
// Currently we want to enforce the presence of only the root configuration file (eirctl.yaml|yml) for analysis
// any referenced imports will be able to find their way and resolve the root correctly in the workspace.
//
// Opening an orphaned configuration file will NOT back resolve the root even if imported
func (s *Server) analysisEntryPath(currentPath string) string {
	if s.rootPath == "" {
		return currentPath
	}

	for _, configFile := range []string{"eirctl.yaml", "eirctl.yml"} {
		rootConfigPath := filepath.Join(s.rootPath, configFile)
		if _, err := s.readFile(rootConfigPath); err == nil {
			return rootConfigPath
		}
	}

	return currentPath
}

func (s *Server) readFile(path string) ([]byte, error) {
	path = filepath.Clean(path)
	if content, ok := s.docs[path]; ok {
		s.log.Debug().Msgf("readFile overlay path=%q", path)
		return []byte(content), nil
	}
	return os.ReadFile(path)
}

func (s *Server) publishDiagnostics(path string) error {
	result, _, err := s.analyze(PathToURI(path))
	if err != nil {
		return err
	}
	byURI := map[string][]lspDiagnostic{}
	for _, diagnostic := range result.Diagnostics {
		uri := PathToURI(diagnostic.URI)
		byURI[uri] = append(byURI[uri], toLSPDiagnostic(diagnostic))
	}
	if _, ok := byURI[PathToURI(path)]; !ok {
		byURI[PathToURI(path)] = []lspDiagnostic{}
	}
	for uri, diagnostics := range byURI {
		if err := s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: uri, Diagnostics: diagnostics}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) respond(id json.RawMessage, result any) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (s *Server) respondError(id json.RawMessage, code int, message string) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "error": &responseError{Code: code, Message: message}})
}

func (s *Server) notify(method string, params any) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (s *Server) write(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(s.writer, RPCMessageFormatter(len(payload), payload))
	return err
}

func (s *Server) dependsOnFallbackCompletions(path string, content []byte, position lspPosition, result analyze.Result) []analyze.CompletionItem {
	scope, ok := dependsOnScopeAt(path, content, position)
	if !ok {
		return nil
	}
	return result.StageCompletionsForScope(scope)
}

func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	headers := map[string]string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, found := strings.Cut(line, ":")
		if found {
			headers[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	length, err := strconv.Atoi(headers["content-length"])
	if err != nil {
		return nil, fmt.Errorf("invalid content length: %w", err)
	}
	payload := make([]byte, length)
	_, err = io.ReadFull(reader, payload)
	return payload, err
}

// HoverMarkdown formats the hover information into a markdown string suitable for display in the LSP client - i.e. an IDE
//
// NOTE: more formatting options may be added in the future.
func HoverMarkdown(hover analyze.Hover) string {
	var buffer bytes.Buffer
	buffer.WriteString("**")
	buffer.WriteString(hover.Title)
	buffer.WriteString("**\n\n")
	buffer.WriteString(hover.Description)
	if hover.Source.Label != "" {
		buffer.WriteString("\n\nSource: `")
		buffer.WriteString(hover.Source.Label)
		buffer.WriteString("`")
	}
	if len(hover.Matches) > 0 {
		buffer.WriteString("\n\nCandidates:\n")
		for _, match := range hover.Matches {
			buffer.WriteString("- `")
			buffer.WriteString(match.Symbol.Name)
			buffer.WriteString("` (")
			buffer.WriteString(string(match.Match))
			buffer.WriteString(") from `")
			buffer.WriteString(match.Symbol.Source.Label)
			buffer.WriteString("`\n")
		}
	}
	return buffer.String()
}

// deriveRootPath determines the root path of the workspace based on the initialization parameters. It checks the RootURI, RootPath, and WorkspaceFolders in order of precedence.
//
// NOTE: this will need to be extended to properly support multi-root workspaces - i.e., when there are multiple workspace folders, the logic here only considers the first one.
// In VSCode this means that in a multi folder/root workspace, only the first workspace folder will be considered as the root path.
func deriveRootPath(params initializeParams) string {
	if params.RootURI != "" {
		if path, err := UriToPath(params.RootURI); err == nil {
			return path
		}
	}
	if params.RootPath != "" {
		return params.RootPath
	}
	if len(params.WorkspaceFolders) > 0 {
		if path, err := UriToPath(params.WorkspaceFolders[0].URI); err == nil {
			return path
		}
	}
	return ""
}

func toProtocolPosition(position lspPosition) langprotocol.Position {
	return langprotocol.Position{Line: position.Line, Character: position.Character}
}

func toLSPLocation(location langprotocol.Location) lspLocation {
	return lspLocation{URI: PathToURI(location.URI), Range: toLSPRange(location.Range)}
}

func toLSPLocations(symbols []langprotocol.Symbol) []lspLocation {
	locations := make([]lspLocation, 0, len(symbols))
	for _, symbol := range symbols {
		locations = append(locations, toLSPLocation(symbol.Location))
	}
	return locations
}

func toLSPReferenceLocations(references []analyze.Reference) []lspLocation {
	locations := make([]lspLocation, 0, len(references))
	for _, reference := range references {
		locations = append(locations, toLSPLocation(reference.Location))
	}
	return locations
}

func toLSPRange(value langprotocol.Range) lspRange {
	return lspRange{Start: lspPosition{Line: value.Start.Line, Character: value.Start.Character}, End: lspPosition{Line: value.End.Line, Character: value.End.Character}}
}

func toLSPDiagnostic(value langprotocol.Diagnostic) lspDiagnostic {
	related := make([]lspDiagnosticRelatedInformation, 0, len(value.Related))
	for _, item := range value.Related {
		related = append(related, lspDiagnosticRelatedInformation{Location: toLSPLocation(item.Location), Message: item.Message})
	}
	return lspDiagnostic{Range: toLSPRange(value.Range), Severity: int(value.Severity), Code: value.Code, Source: value.Source, Message: value.Message, RelatedInformation: related}
}

func toLSPCompletionItems(items []analyze.CompletionItem) []lspCompletionItem {
	converted := make([]lspCompletionItem, 0, len(items))
	for index, item := range items {
		converted = append(converted, lspCompletionItem{
			Label:         item.Label,
			Kind:          symbolKindToCompletionKind(item.Kind),
			Detail:        item.Detail,
			InsertText:    item.InsertText,
			Documentation: &markupContent{Kind: "markdown", Value: fmt.Sprintf("Source: `%s`\n\nMatch: `%s`", item.Source.Label, item.Match)},
			SortText:      fmt.Sprintf("%04d-%s", index, item.Label),
		})
	}
	return converted
}

func toLSPDocumentSymbols(result analyze.Result, path string) []lspDocumentSymbol {
	var symbols []lspDocumentSymbol
	for _, symbol := range result.Symbols {
		if symbol.Location.URI != path {
			continue
		}
		item := lspDocumentSymbol{Name: symbol.Name, Detail: symbol.Detail, Kind: symbolKindToDocumentSymbolKind(symbol.Kind), Range: toLSPRange(symbol.Location.Range), SelectionRange: toLSPRange(symbol.Location.Range)}
		symbols = append(symbols, item)
	}
	return symbols
}

func dependsOnScopeAt(path string, content []byte, position lspPosition) (string, bool) {
	lines := strings.Split(string(content), "\n")
	if position.Line < 0 || position.Line >= len(lines) {
		return "", false
	}

	for lineIndex := position.Line; lineIndex >= 0; lineIndex-- {
		trimmed := strings.TrimSpace(lines[lineIndex])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "depends_on:") {
			return pipelineScopeFromLines(path, lines, lineIndex)
		}
	}

	return "", false
}

func pipelineScopeFromLines(path string, lines []string, dependsOnLine int) (string, bool) {
	for lineIndex := dependsOnLine - 1; lineIndex >= 0; lineIndex-- {
		line := lines[lineIndex]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "pipelines:" {
			return "", false
		}
		if countLeadingSpaces(line) == 2 && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			pipelineName := strings.TrimSuffix(trimmed, ":")
			return pipelineScopeForPath(path, pipelineName), true
		}
	}
	return "", false
}

func pipelineScopeForPath(path string, pipelineName string) string {
	return filepath.Clean(path) + "\x00" + pipelineName
}

func countLeadingSpaces(value string) int {
	count := 0
	for count < len(value) && value[count] == ' ' {
		count++
	}
	return count
}

func symbolKindToCompletionKind(kind langprotocol.SymbolKind) int {
	switch kind {
	case langprotocol.SymbolKindTask, langprotocol.SymbolKindPipeline:
		return 3
	case langprotocol.SymbolKindContext:
		return 6
	default:
		return 1
	}
}

func symbolKindToDocumentSymbolKind(kind langprotocol.SymbolKind) int {
	switch kind {
	case langprotocol.SymbolKindTask, langprotocol.SymbolKindPipeline:
		return 12
	case langprotocol.SymbolKindContext:
		return 5
	case langprotocol.SymbolKindWatcher:
		return 13
	default:
		return 13
	}
}

func UriToPath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Scheme == "file" {
		path := parsed.Path
		if path == "" {
			path = value
		}
		if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		return filepath.Clean(path), nil
	}
	return "", fmt.Errorf("%w: %s", ErrUnsupportedURIScheme, parsed.Scheme)
}

func PathToURI(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	if len(path) > 0 && path[0] != '/' {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}
