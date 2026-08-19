package analyze

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Ensono/eirctl/lang/ast"
	"github.com/Ensono/eirctl/lang/protocol"
	"github.com/Ensono/eirctl/lang/workspace"
	"gopkg.in/yaml.v3"
)

const diagnosticSource = "eirctl-lang"

type Options struct {
	BaseDir  string
	HomeDir  string
	ReadFile func(path string) ([]byte, error)
}

type Import struct {
	Raw      string
	Location protocol.Location
	Resolved workspace.ResolvedImport
	Source   protocol.DocumentSource
}

type Reference struct {
	Name     string
	Kind     protocol.SymbolKind
	Field    string
	Location protocol.Location
	Source   protocol.DocumentSource
}

type MatchKind string

const (
	MatchKindExact       MatchKind = "exact"
	MatchKindPrefix      MatchKind = "prefix"
	MatchKindSubstring   MatchKind = "substring"
	MatchKindSubsequence MatchKind = "subsequence"
)

type DefinitionMatch struct {
	Symbol protocol.Symbol
	Score  int
	Match  MatchKind
}

type Hover struct {
	Title       string
	Description string
	Source      protocol.DocumentSource
	Matches     []DefinitionMatch
}

type CompletionItem struct {
	Label      string
	Detail     string
	InsertText string
	Kind       protocol.SymbolKind
	Source     protocol.DocumentSource
	Score      int
	Match      MatchKind
}

type Result struct {
	Symbols     []protocol.Symbol
	References  []Reference
	Imports     []Import
	Diagnostics []protocol.Diagnostic
	Sources     map[string]protocol.DocumentSource
}

func AnalyzeDocument(doc *ast.Document, opts Options) Result {
	return analyzeDocuments([]*ast.Document{doc}, nil, map[string]protocol.DocumentSource{
		filepath.Clean(doc.URI): {
			URI:      doc.URI,
			Path:     filepath.Clean(doc.URI),
			Original: doc.URI,
			Label:    filepath.Clean(doc.URI),
		},
	}, opts)
}

func AnalyzeWorkspace(doc *ast.Document, opts Options) Result {
	snapshot := workspace.LoadDocuments(doc, workspace.LoadOptions{HomeDir: opts.HomeDir, ReadFile: opts.ReadFile})
	return analyzeDocuments(snapshot.Documents, snapshot.Issues, snapshot.Sources, opts)
}

func analyzeDocuments(docs []*ast.Document, issues []workspace.LoadIssue, sources map[string]protocol.DocumentSource, opts Options) Result {
	result := Result{Sources: map[string]protocol.DocumentSource{}}
	if len(docs) == 0 {
		return result
	}
	for uri, source := range sources {
		result.Sources[uri] = source
	}

	for _, issue := range issues {
		result.Diagnostics = append(result.Diagnostics, protocol.Diagnostic{
			URI:      issue.Import.Location.URI,
			Range:    issue.Import.Location.Range,
			Severity: protocol.SeverityError,
			Code:     "import-load-failed",
			Source:   diagnosticSource,
			Message:  issue.Message,
		})
	}

	if opts.BaseDir == "" {
		opts.BaseDir = filepath.Dir(docs[0].URI)
	}

	definitions := map[protocol.SymbolKind]map[string]protocol.Symbol{
		protocol.SymbolKindTask:     {},
		protocol.SymbolKindPipeline: {},
		protocol.SymbolKindContext:  {},
		protocol.SymbolKindWatcher:  {},
	}

	for _, doc := range docs {
		docOpts := opts
		docOpts.BaseDir = filepath.Dir(doc.URI)
		docSource := documentSource(result.Sources, doc.URI)

		collectDefinitions(doc, &result, definitions, docSource, "tasks", protocol.SymbolKindTask)
		collectDefinitions(doc, &result, definitions, docSource, "pipelines", protocol.SymbolKindPipeline)
		collectDefinitions(doc, &result, definitions, docSource, "contexts", protocol.SymbolKindContext)
		collectDefinitions(doc, &result, definitions, docSource, "watchers", protocol.SymbolKindWatcher)
		collectImports(doc, &result, docSource, docOpts)
		collectTaskReferences(doc, &result, docSource)
		collectWatcherReferences(doc, &result, docSource)
		collectPipelineReferences(doc, &result, docSource)
	}

	validateReferences(&result, definitions)

	return result
}

func documentSource(sources map[string]protocol.DocumentSource, uri string) protocol.DocumentSource {
	if source, ok := sources[filepath.Clean(uri)]; ok {
		return source
	}
	return protocol.DocumentSource{URI: uri, Path: filepath.Clean(uri), Original: uri}
}

func collectDefinitions(doc *ast.Document, result *Result, definitions map[protocol.SymbolKind]map[string]protocol.Symbol, source protocol.DocumentSource, section string, kind protocol.SymbolKind) {
	_, sectionNode, ok := doc.TopLevelSection(section)
	if !ok {
		return
	}

	for _, entry := range ast.MappingEntries(sectionNode) {
		symbol := protocol.Symbol{
			Name:     entry.Key.Value,
			Kind:     kind,
			Detail:   section,
			Location: doc.Location(entry.Key),
			Source:   source,
		}
		if _, exists := definitions[kind][symbol.Name]; exists {
			existing := definitions[kind][symbol.Name]
			result.Symbols = append(result.Symbols, symbol)
			result.Diagnostics = append(result.Diagnostics, protocol.Diagnostic{
				URI:      symbol.Location.URI,
				Range:    symbol.Location.Range,
				Severity: protocol.SeverityError,
				Code:     "duplicate-symbol",
				Source:   diagnosticSource,
				Message:  fmt.Sprintf("duplicate %s %q", kind, symbol.Name),
				Related: []protocol.RelatedInformation{{
					Location: existing.Location,
					Message:  fmt.Sprintf("first %s defined here", kind),
				}},
			})
			continue
		}
		definitions[kind][symbol.Name] = symbol
		result.Symbols = append(result.Symbols, symbol)
	}
}

func collectImports(doc *ast.Document, result *Result, source protocol.DocumentSource, opts Options) {
	_, importNode, ok := doc.TopLevelSection("import")
	if !ok || importNode.Kind != yaml.SequenceNode {
		return
	}

	for _, item := range ast.SequenceItems(importNode) {
		rawNode := item
		raw := ""

		switch item.Kind {
		case yaml.ScalarNode:
			raw = item.Value
		case yaml.MappingNode:
			_, srcNode, found := ast.LookupMappingValue(item, "src")
			if !found {
				continue
			}
			rawNode = srcNode
			raw = srcNode.Value
		default:
			continue
		}

		result.Imports = append(result.Imports, Import{
			Raw:      raw,
			Location: doc.Location(rawNode),
			Resolved: workspace.ResolveImport(opts.BaseDir, opts.HomeDir, raw),
			Source:   source,
		})
	}
}

func collectTaskReferences(doc *ast.Document, result *Result, source protocol.DocumentSource) {
	_, tasksNode, ok := doc.TopLevelSection("tasks")
	if !ok {
		return
	}

	for _, taskEntry := range ast.MappingEntries(tasksNode) {
		_, contextNode, found := ast.LookupMappingValue(taskEntry.Value, "context")
		if !found || contextNode.Kind != yaml.ScalarNode || contextNode.Value == "" {
			continue
		}
		result.References = append(result.References, Reference{
			Name:     contextNode.Value,
			Kind:     protocol.SymbolKindContext,
			Field:    "tasks.context",
			Location: doc.Location(contextNode),
			Source:   source,
		})
	}
}

func collectWatcherReferences(doc *ast.Document, result *Result, source protocol.DocumentSource) {
	_, watchersNode, ok := doc.TopLevelSection("watchers")
	if !ok {
		return
	}

	for _, watcherEntry := range ast.MappingEntries(watchersNode) {
		_, taskNode, found := ast.LookupMappingValue(watcherEntry.Value, "task")
		if !found || taskNode.Kind != yaml.ScalarNode || taskNode.Value == "" {
			continue
		}
		result.References = append(result.References, Reference{
			Name:     taskNode.Value,
			Kind:     protocol.SymbolKindTask,
			Field:    "watchers.task",
			Location: doc.Location(taskNode),
			Source:   source,
		})
	}
}

func collectPipelineReferences(doc *ast.Document, result *Result, source protocol.DocumentSource) {
	_, pipelinesNode, ok := doc.TopLevelSection("pipelines")
	if !ok {
		return
	}

	for _, pipelineEntry := range ast.MappingEntries(pipelinesNode) {
		for _, stageNode := range ast.SequenceItems(pipelineEntry.Value) {
			collectPipelineStageReference(doc, result, source, stageNode, "task", protocol.SymbolKindTask)
			collectPipelineStageReference(doc, result, source, stageNode, "pipeline", protocol.SymbolKindPipeline)
		}
	}
}

func collectPipelineStageReference(doc *ast.Document, result *Result, source protocol.DocumentSource, stageNode *yaml.Node, field string, kind protocol.SymbolKind) {
	_, valueNode, found := ast.LookupMappingValue(stageNode, field)
	if !found || valueNode.Kind != yaml.ScalarNode || valueNode.Value == "" {
		return
	}
	result.References = append(result.References, Reference{
		Name:     valueNode.Value,
		Kind:     kind,
		Field:    "pipelines." + field,
		Location: doc.Location(valueNode),
		Source:   source,
	})
}

func (r Result) Definitions(name string, kind protocol.SymbolKind) []protocol.Symbol {
	var matches []protocol.Symbol
	for _, symbol := range r.Symbols {
		if symbol.Kind == kind && symbol.Name == name {
			matches = append(matches, symbol)
		}
	}
	return matches
}

func (r Result) DefinitionFor(reference Reference) (protocol.Symbol, bool) {
	matches := r.DefinitionMatchesFor(reference)
	if len(matches) > 0 {
		return matches[0].Symbol, true
	}
	return protocol.Symbol{}, false
}

func (r Result) ReferencesFor(name string, kind protocol.SymbolKind) []Reference {
	var matches []Reference
	for _, reference := range r.References {
		if reference.Kind == kind && reference.Name == name {
			matches = append(matches, reference)
		}
	}
	return matches
}

func (r Result) ReferencesForSymbol(symbol protocol.Symbol) []Reference {
	return r.ReferencesFor(symbol.Name, symbol.Kind)
}

func (r Result) DefinitionCandidatesFor(reference Reference) []protocol.Symbol {
	matches := r.DefinitionMatchesFor(reference)
	definitions := make([]protocol.Symbol, 0, len(matches))
	for _, match := range matches {
		definitions = append(definitions, match.Symbol)
	}
	return definitions
}

func (r Result) DefinitionMatchesFor(reference Reference) []DefinitionMatch {
	if definitions := r.Definitions(reference.Name, reference.Kind); len(definitions) > 0 {
		matches := make([]DefinitionMatch, 0, len(definitions))
		for _, definition := range definitions {
			matches = append(matches, DefinitionMatch{Symbol: definition, Score: 400, Match: MatchKindExact})
		}
		return matches
	}
	return r.FuzzyDefinitions(reference.Name, reference.Kind)
}

func (r Result) FuzzyDefinitions(name string, kind protocol.SymbolKind) []DefinitionMatch {
	type candidate struct {
		symbol protocol.Symbol
		score  int
		match  MatchKind
	}

	query := strings.ToLower(strings.TrimSpace(name))
	if query == "" {
		return nil
	}

	var candidates []candidate
	seen := map[string]bool{}
	for _, symbol := range r.Symbols {
		if symbol.Kind != kind {
			continue
		}
		key := symbol.Kind.String() + "\x00" + symbol.Name + "\x00" + symbol.Location.URI + fmt.Sprintf(":%d:%d", symbol.Location.Range.Start.Line, symbol.Location.Range.Start.Character)
		if seen[key] {
			continue
		}
		score, match, ok := fuzzyMatchScore(query, strings.ToLower(symbol.Name))
		if !ok {
			continue
		}
		seen[key] = true
		candidates = append(candidates, candidate{symbol: symbol, score: score, match: match})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].symbol.Name != candidates[j].symbol.Name {
			return candidates[i].symbol.Name < candidates[j].symbol.Name
		}
		if candidates[i].symbol.Source.Label != candidates[j].symbol.Source.Label {
			return candidates[i].symbol.Source.Label < candidates[j].symbol.Source.Label
		}
		return candidates[i].symbol.Location.Range.Start.Character < candidates[j].symbol.Location.Range.Start.Character
	})

	definitions := make([]DefinitionMatch, 0, len(candidates))
	for _, candidate := range candidates {
		definitions = append(definitions, DefinitionMatch{Symbol: candidate.symbol, Score: candidate.score, Match: candidate.match})
	}
	return definitions
}

func (r Result) SymbolAt(uri string, position protocol.Position) (protocol.Symbol, bool) {
	for _, symbol := range r.Symbols {
		if symbol.Location.URI == uri && symbol.Location.Range.Contains(position) {
			return symbol, true
		}
	}
	return protocol.Symbol{}, false
}

func (r Result) ReferenceAt(uri string, position protocol.Position) (Reference, bool) {
	for _, reference := range r.References {
		if reference.Location.URI == uri && reference.Location.Range.Contains(position) {
			return reference, true
		}
	}
	return Reference{}, false
}

func (r Result) DefinitionsAt(uri string, position protocol.Position) []protocol.Symbol {
	if reference, ok := r.ReferenceAt(uri, position); ok {
		return r.DefinitionCandidatesFor(reference)
	}
	if symbol, ok := r.SymbolAt(uri, position); ok {
		return r.Definitions(symbol.Name, symbol.Kind)
	}
	return nil
}

func (r Result) DefinitionMatchesAt(uri string, position protocol.Position) []DefinitionMatch {
	if reference, ok := r.ReferenceAt(uri, position); ok {
		return r.DefinitionMatchesFor(reference)
	}
	if symbol, ok := r.SymbolAt(uri, position); ok {
		definitions := r.Definitions(symbol.Name, symbol.Kind)
		matches := make([]DefinitionMatch, 0, len(definitions))
		for _, definition := range definitions {
			matches = append(matches, DefinitionMatch{Symbol: definition, Score: 400, Match: MatchKindExact})
		}
		return matches
	}
	return nil
}

func (r Result) ReferencesAt(uri string, position protocol.Position) []Reference {
	if symbol, ok := r.SymbolAt(uri, position); ok {
		return r.ReferencesForSymbol(symbol)
	}
	if reference, ok := r.ReferenceAt(uri, position); ok {
		if definition, found := r.DefinitionFor(reference); found {
			return r.ReferencesForSymbol(definition)
		}
		return []Reference{reference}
	}
	return nil
}

func (r Result) HoverAt(uri string, position protocol.Position) (Hover, bool) {
	if symbol, ok := r.SymbolAt(uri, position); ok {
		references := r.ReferencesForSymbol(symbol)
		return Hover{
			Title:       fmt.Sprintf("%s %s", symbol.Kind, symbol.Name),
			Description: fmt.Sprintf("Defined in %s. %d reference(s).", symbol.Source.Label, len(references)),
			Source:      symbol.Source,
			Matches:     []DefinitionMatch{{Symbol: symbol, Score: 400, Match: MatchKindExact}},
		}, true
	}
	if reference, ok := r.ReferenceAt(uri, position); ok {
		matches := r.DefinitionMatchesFor(reference)
		message := fmt.Sprintf("Reference %s in %s.", reference.Name, reference.Field)
		if len(matches) > 0 {
			message = fmt.Sprintf("Reference %s in %s resolves to %d candidate(s).", reference.Name, reference.Field, len(matches))
		}
		return Hover{
			Title:       fmt.Sprintf("%s reference %s", reference.Kind, reference.Name),
			Description: message,
			Source:      reference.Source,
			Matches:     matches,
		}, true
	}
	return Hover{}, false
}

func (r Result) CompletionsAt(uri string, position protocol.Position) []CompletionItem {
	if reference, ok := r.ReferenceAt(uri, position); ok {
		matches := r.DefinitionMatchesFor(reference)
		items := make([]CompletionItem, 0, len(matches))
		for _, match := range matches {
			items = append(items, CompletionItem{
				Label:      match.Symbol.Name,
				Detail:     fmt.Sprintf("%s from %s (%s)", match.Symbol.Kind, match.Symbol.Source.Label, match.Match),
				InsertText: match.Symbol.Name,
				Kind:       match.Symbol.Kind,
				Source:     match.Symbol.Source,
				Score:      match.Score,
				Match:      match.Match,
			})
		}
		return items
	}
	return nil
}

func validateReferences(result *Result, definitions map[protocol.SymbolKind]map[string]protocol.Symbol) {
	for _, reference := range result.References {
		if _, exists := definitions[reference.Kind][reference.Name]; exists {
			continue
		}
		result.Diagnostics = append(result.Diagnostics, protocol.Diagnostic{
			URI:      reference.Location.URI,
			Range:    reference.Location.Range,
			Severity: protocol.SeverityError,
			Code:     "unresolved-reference",
			Source:   diagnosticSource,
			Message:  fmt.Sprintf("unresolved %s reference %q in %s", reference.Kind, reference.Name, reference.Field),
		})
	}
}

func fuzzyMatchScore(query, candidate string) (int, MatchKind, bool) {
	if query == candidate {
		return 400, MatchKindExact, true
	}
	if strings.HasPrefix(candidate, query) {
		return 300, MatchKindPrefix, true
	}
	if strings.Contains(candidate, query) {
		return 200, MatchKindSubstring, true
	}
	if isSubsequence(query, candidate) {
		return 100, MatchKindSubsequence, true
	}
	return 0, "", false
}

func isSubsequence(query, candidate string) bool {
	if query == "" {
		return true
	}
	queryIndex := 0
	for _, character := range candidate {
		if rune(query[queryIndex]) == character {
			queryIndex++
			if queryIndex == len(query) {
				return true
			}
		}
	}
	return false
}
