package ast

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Ensono/eirctl/lang/protocol"
	"gopkg.in/yaml.v3"
)

type Document struct {
	URI    string
	Source []byte
	Root   *yaml.Node
}

type MappingEntry struct {
	Key   *yaml.Node
	Value *yaml.Node
}

const diagnosticSource = "eirctl-lang"

func Parse(uri string, content []byte) (*Document, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	return &Document{
		URI:    uri,
		Source: append([]byte(nil), content...),
		Root:   &root,
	}, nil
}

func ParseLenient(uri string, content []byte) (*Document, error) {
	doc, _, err := ParseRecovering(uri, content)
	return doc, err
}

func ParseRecovering(uri string, content []byte) (*Document, []protocol.Diagnostic, error) {
	diagnostics := []protocol.Diagnostic{}
	current := append([]byte(nil), content...)

	for attempts := 0; attempts < 64; attempts++ {
		doc, err := Parse(uri, current)
		if err == nil {
			return doc, diagnostics, nil
		}

		line, ok := parseYAMLParseErrorLine(err)
		if !ok {
			return emptyDocument(uri, content), diagnostics, err
		}

		diagnostics = append(diagnostics, parseDiagnostic(uri, line, err.Error()))
		next := blankYAMLLines(current, line)
		if bytes.Equal(next, current) {
			return emptyDocument(uri, content), diagnostics, err
		}
		current = next
	}

	return emptyDocument(uri, content), diagnostics, fmt.Errorf("parse yaml: unable to recover")
}

func (d *Document) ContentRoot() *yaml.Node {
	if d == nil || d.Root == nil {
		return nil
	}
	if d.Root.Kind == yaml.DocumentNode && len(d.Root.Content) > 0 {
		return d.Root.Content[0]
	}
	return d.Root
}

func (d *Document) TopLevelMap() *yaml.Node {
	root := d.ContentRoot()
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	return root
}

func (d *Document) TopLevelSection(name string) (*yaml.Node, *yaml.Node, bool) {
	return LookupMappingValue(d.TopLevelMap(), name)
}

func (d *Document) Location(node *yaml.Node) protocol.Location {
	return protocol.Location{
		URI:   d.URI,
		Range: NodeRange(node),
	}
}

func MappingEntries(mapping *yaml.Node) []MappingEntry {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}

	entries := make([]MappingEntry, 0, len(mapping.Content)/2)
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		entries = append(entries, MappingEntry{
			Key:   mapping.Content[index],
			Value: mapping.Content[index+1],
		})
	}
	return entries
}

func LookupMappingValue(mapping *yaml.Node, key string) (*yaml.Node, *yaml.Node, bool) {
	for _, entry := range MappingEntries(mapping) {
		if entry.Key.Value == key {
			return entry.Key, entry.Value, true
		}
	}
	return nil, nil, false
}

func SequenceItems(node *yaml.Node) []*yaml.Node {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return node.Content
}

func ScalarValue(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

func NodeRange(node *yaml.Node) protocol.Range {
	if node == nil {
		return protocol.Range{}
	}

	start := protocol.Position{}
	if node.Line > 0 {
		start.Line = node.Line - 1
	}
	if node.Column > 0 {
		start.Character = node.Column - 1
	}

	width := 1
	if node.Kind == yaml.ScalarNode && node.Value != "" {
		width = utf8.RuneCountInString(node.Value)
	}

	return protocol.Range{
		Start: start,
		End: protocol.Position{
			Line:      start.Line,
			Character: start.Character + width,
		},
	}
}

func parseYAMLParseErrorLine(err error) (int, bool) {
	message := err.Error()
	index := strings.Index(message, "line ")
	if index == -1 {
		return 0, false
	}
	index += len("line ")
	end := index
	for end < len(message) && message[end] >= '0' && message[end] <= '9' {
		end++
	}
	if end == index {
		return 0, false
	}
	line, convErr := strconv.Atoi(message[index:end])
	if convErr != nil || line <= 0 {
		return 0, false
	}
	return line, true
}

func blankYAMLLines(content []byte, lineNumber int) []byte {
	lines := strings.Split(string(content), "\n")
	if lineNumber < 1 || lineNumber > len(lines) {
		return content
	}
	lines[lineNumber-1] = blankYAMLLine(lines[lineNumber-1])
	return []byte(strings.Join(lines, "\n"))
}

func blankYAMLLine(line string) string {
	trimmedLeft := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmedLeft)]
	if indent == "" {
		return "# eirctl parse recovery"
	}
	return indent + "# eirctl parse recovery"
}

func emptyDocument(uri string, content []byte) *Document {
	root := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	return &Document{URI: uri, Source: append([]byte(nil), content...), Root: &root}
}

func parseDiagnostic(uri string, line int, message string) protocol.Diagnostic {
	return protocol.Diagnostic{
		URI:      uri,
		Range:    protocol.Range{Start: protocol.Position{Line: line - 1}, End: protocol.Position{Line: line - 1, Character: 1}},
		Code:     "yaml-parse-error",
		Source:   diagnosticSource,
		Severity: protocol.SeverityError,
		Message:  message,
	}
}
