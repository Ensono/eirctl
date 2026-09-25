package protocol_test

import (
	"testing"

	"github.com/Ensono/eirctl/lang/protocol"
)

func TestRange_Contains(t *testing.T) {
	ttests := map[string]struct {
		r        protocol.Range
		position protocol.Position
		want     bool
	}{
		"position before start line": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 10, Character: 0},
			},
			position: protocol.Position{Line: 4, Character: 0},
			want:     false,
		},
		"position after end line": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 10, Character: 0},
			},
			position: protocol.Position{Line: 11, Character: 0},
			want:     false,
		},
		"position on start line before start character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 0},
			},
			position: protocol.Position{Line: 5, Character: 5},
			want:     false,
		},
		"position on start line at start character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 0},
			},
			position: protocol.Position{Line: 5, Character: 10},
			want:     true,
		},
		"position on start line after start character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 0},
			},
			position: protocol.Position{Line: 5, Character: 20},
			want:     true,
		},
		"position on end line before end character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 10, Character: 15},
			},
			position: protocol.Position{Line: 10, Character: 5},
			want:     true,
		},
		"position on end line at end character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 10, Character: 15},
			},
			position: protocol.Position{Line: 10, Character: 15},
			want:     true,
		},
		"position on end line after end character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 0},
				End:   protocol.Position{Line: 10, Character: 15},
			},
			position: protocol.Position{Line: 10, Character: 20},
			want:     false,
		},
		"position on line strictly between start and end": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 10},
				End:   protocol.Position{Line: 10, Character: 15},
			},
			position: protocol.Position{Line: 7, Character: 0},
			want:     true,
		},
		"single line range, position within character bounds": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 5},
				End:   protocol.Position{Line: 5, Character: 15},
			},
			position: protocol.Position{Line: 5, Character: 10},
			want:     true,
		},
		"single line range, position before start character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 5},
				End:   protocol.Position{Line: 5, Character: 15},
			},
			position: protocol.Position{Line: 5, Character: 2},
			want:     false,
		},
		"single line range, position after end character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 5},
				End:   protocol.Position{Line: 5, Character: 15},
			},
			position: protocol.Position{Line: 5, Character: 20},
			want:     false,
		},
		"single line range, position equals both start and end character": {
			r: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 5},
				End:   protocol.Position{Line: 5, Character: 5},
			},
			position: protocol.Position{Line: 5, Character: 5},
			want:     true,
		},
	}

	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			got := tt.r.Contains(tt.position)
			if got != tt.want {
				t.Errorf("Range.Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SymbolKindString(t *testing.T) {
    ttests := map[string]struct {
        kind protocol.SymbolKind
        want string
    }{
        "task": {
            kind: protocol.SymbolKindTask,
            want: "task",
        },
        "pipeline": {
            kind: protocol.SymbolKindPipeline,
            want: "pipeline",
        },
        "context": {
            kind: protocol.SymbolKindContext,
            want: "context",
        },
        "watcher": {
            kind: protocol.SymbolKindWatcher,
            want: "watcher",
        },
        "stage": {
            kind: protocol.SymbolKindStage,
            want: "stage",
        },
        "import": {
            kind: protocol.SymbolKindImport,
            want: "import",
        },
    }
    for name, tt := range ttests {
        t.Run(name, func(t *testing.T) {
            got := tt.kind.String()
            if got != tt.want {
                t.Errorf("SymbolKind.String() = %v, want %v", got, tt.want)
            }
        })
    }
}