package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
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
)

type Server struct {
	reader   *bufio.Reader
	writer   io.Writer
	homeDir  string
	rootPath string
	docs     map[string]string
	closed   bool
}

func NewServer(in io.Reader, out io.Writer) (*Server, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Server{
		reader:  bufio.NewReader(in),
		writer:  out,
		homeDir: homeDir,
		docs:    map[string]string{},
	}, nil
}

func (s *Server) Serve() error {
	for !s.closed {
		payload, err := readMessage(s.reader)
		if err != nil {
			if err == io.EOF {
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

type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type responseEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeParams struct {
	RootURI          string `json:"rootUri"`
	RootPath         string `json:"rootPath"`
	WorkspaceFolders []struct {
		URI string `json:"uri"`
	} `json:"workspaceFolders"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenTextDocumentParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type didChangeTextDocumentParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type didCloseTextDocumentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     lspPosition            `json:"position"`
}

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     lspPosition            `json:"position"`
	Context      struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type completionParams = textDocumentPositionParams
type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type lspDiagnosticRelatedInformation struct {
	Location lspLocation `json:"location"`
	Message  string      `json:"message"`
}

type lspDiagnostic struct {
	Range              lspRange                          `json:"range"`
	Severity           int                               `json:"severity,omitempty"`
	Code               string                            `json:"code,omitempty"`
	Source             string                            `json:"source,omitempty"`
	Message            string                            `json:"message"`
	RelatedInformation []lspDiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
}

type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type lspHover struct {
	Contents markupContent `json:"contents"`
	Range    *lspRange     `json:"range,omitempty"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type lspCompletionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind,omitempty"`
	Detail        string         `json:"detail,omitempty"`
	InsertText    string         `json:"insertText,omitempty"`
	Documentation *markupContent `json:"documentation,omitempty"`
	SortText      string         `json:"sortText,omitempty"`
}

type lspDocumentSymbol struct {
	Name           string   `json:"name"`
	Detail         string   `json:"detail,omitempty"`
	Kind           int      `json:"kind"`
	Range          lspRange `json:"range"`
	SelectionRange lspRange `json:"selectionRange"`
}

func (s *Server) handleMessage(payload []byte) error {
	var req requestEnvelope
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	switch req.Method {
	case "initialize":
		var params initializeParams
		_ = json.Unmarshal(req.Params, &params)
		s.rootPath = deriveRootPath(params)
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
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		path, err := uriToPath(params.TextDocument.URI)
		if err != nil {
			return err
		}
		s.docs[path] = params.TextDocument.Text
		return s.publishDiagnostics(path)
	case "textDocument/didChange":
		var params didChangeTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		path, err := uriToPath(params.TextDocument.URI)
		if err != nil {
			return err
		}
		if len(params.ContentChanges) > 0 {
			s.docs[path] = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
		return s.publishDiagnostics(path)
	case "textDocument/didClose":
		var params didCloseTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		path, err := uriToPath(params.TextDocument.URI)
		if err != nil {
			return err
		}
		delete(s.docs, path)
		return s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: params.TextDocument.URI, Diagnostics: nil})
	case "textDocument/definition":
		var params textDocumentPositionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, -32603, err.Error())
		}
		defs := result.DefinitionsAt(path, toProtocolPosition(params.Position))
		return s.respond(req.ID, toLSPLocations(defs))
	case "textDocument/references":
		var params referenceParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, -32603, err.Error())
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
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, -32603, err.Error())
		}
		hover, ok := result.HoverAt(path, toProtocolPosition(params.Position))
		if !ok {
			return s.respond(req.ID, nil)
		}
		return s.respond(req.ID, lspHover{Contents: markupContent{Kind: "markdown", Value: hoverMarkdown(hover)}})
	case "textDocument/completion":
		var params completionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, -32603, err.Error())
		}
		items := result.CompletionsAt(path, toProtocolPosition(params.Position))
		return s.respond(req.ID, toLSPCompletionItems(items))
	case "textDocument/documentSymbol":
		var params documentSymbolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return err
		}
		result, path, err := s.analyze(params.TextDocument.URI)
		if err != nil {
			return s.respondError(req.ID, -32603, err.Error())
		}
		return s.respond(req.ID, toLSPDocumentSymbols(result, path))
	default:
		if len(req.ID) > 0 {
			return s.respond(req.ID, nil)
		}
		return nil
	}
}

func (s *Server) analyze(uri string) (analyze.Result, string, error) {
	path, err := uriToPath(uri)
	if err != nil {
		return analyze.Result{}, "", err
	}
	content, err := s.readFile(path)
	if err != nil {
		return analyze.Result{}, "", err
	}
	doc, err := ast.Parse(path, content)
	if err != nil {
		return analyze.Result{}, "", err
	}
	result := analyze.AnalyzeWorkspace(doc, analyze.Options{HomeDir: s.homeDir, ReadFile: s.readFile, BaseDir: filepath.Dir(path)})
	return result, path, nil
}

func (s *Server) readFile(path string) ([]byte, error) {
	path = filepath.Clean(path)
	if content, ok := s.docs[path]; ok {
		return []byte(content), nil
	}
	return os.ReadFile(path)
}

func (s *Server) publishDiagnostics(path string) error {
	result, _, err := s.analyze(pathToURI(path))
	if err != nil {
		return err
	}
	byURI := map[string][]lspDiagnostic{}
	for _, diagnostic := range result.Diagnostics {
		uri := pathToURI(diagnostic.URI)
		byURI[uri] = append(byURI[uri], toLSPDiagnostic(diagnostic))
	}
	if _, ok := byURI[pathToURI(path)]; !ok {
		byURI[pathToURI(path)] = nil
	}
	for uri, diagnostics := range byURI {
		if err := s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: uri, Diagnostics: diagnostics}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) respond(id json.RawMessage, result any) error {
	return s.write(responseEnvelope{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) respondError(id json.RawMessage, code int, message string) error {
	return s.write(responseEnvelope{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: message}})
}

func (s *Server) notify(method string, params any) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (s *Server) write(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.writer, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
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

func deriveRootPath(params initializeParams) string {
	if params.RootURI != "" {
		if path, err := uriToPath(params.RootURI); err == nil {
			return path
		}
	}
	if params.RootPath != "" {
		return params.RootPath
	}
	if len(params.WorkspaceFolders) > 0 {
		if path, err := uriToPath(params.WorkspaceFolders[0].URI); err == nil {
			return path
		}
	}
	return ""
}

func toProtocolPosition(position lspPosition) langprotocol.Position {
	return langprotocol.Position{Line: position.Line, Character: position.Character}
}

func toLSPLocation(location langprotocol.Location) lspLocation {
	return lspLocation{URI: pathToURI(location.URI), Range: toLSPRange(location.Range)}
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

func hoverMarkdown(hover analyze.Hover) string {
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

func uriToPath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Scheme == "file" {
		path := parsed.Path
		if path == "" {
			path = value
		}
		return filepath.Clean(path), nil
	}
	return "", fmt.Errorf("unsupported URI scheme: %s", parsed.Scheme)
}

func pathToURI(path string) string {
	path = filepath.Clean(path)
	return (&url.URL{Scheme: "file", Path: path}).String()
}
