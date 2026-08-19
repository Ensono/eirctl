package protocol

type Position struct {
	Line      int
	Character int
}

type Range struct {
	Start Position
	End   Position
}

func (r Range) Contains(position Position) bool {
	if position.Line < r.Start.Line || position.Line > r.End.Line {
		return false
	}
	if position.Line == r.Start.Line && position.Character < r.Start.Character {
		return false
	}
	if position.Line == r.End.Line && position.Character > r.End.Character {
		return false
	}
	return true
}

type Location struct {
	URI   string
	Range Range
}

type DocumentSource struct {
	URI       string
	Path      string
	Original  string
	Label     string
	FromCache bool
}

type RelatedInformation struct {
	Location Location
	Message  string
}

type DiagnosticSeverity int

const (
	SeverityError DiagnosticSeverity = iota + 1
	SeverityWarning
	SeverityInformation
	SeverityHint
)

type Diagnostic struct {
	URI      string
	Range    Range
	Severity DiagnosticSeverity
	Code     string
	Source   string
	Message  string
	Related  []RelatedInformation
}

type SymbolKind string

func (s SymbolKind) String() string {
	return string(s)
}

const (
	SymbolKindTask     SymbolKind = "task"
	SymbolKindPipeline SymbolKind = "pipeline"
	SymbolKindContext  SymbolKind = "context"
	SymbolKindWatcher  SymbolKind = "watcher"
	SymbolKindStage    SymbolKind = "stage"
	SymbolKindImport   SymbolKind = "import"
)

type Symbol struct {
	Name     string
	Kind     SymbolKind
	Detail   string
	Location Location
	Source   DocumentSource
}
