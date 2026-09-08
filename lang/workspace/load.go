package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Ensono/eirctl/lang/ast"
	"github.com/Ensono/eirctl/lang/protocol"
	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	HomeDir  string
	ReadFile func(path string) ([]byte, error)
}

type ImportReference struct {
	Raw          string
	Location     protocol.Location
	IsFileImport bool
	Resolved     ResolvedImport
}

type LoadIssue struct {
	Import  ImportReference
	Message string
}

type Snapshot struct {
	Documents []*ast.Document
	Issues    []LoadIssue
	Sources   map[string]protocol.DocumentSource
}

func sourceLabel(path, original string) string {
	if original == "" || original == path {
		return path
	}
	return fmt.Sprintf("%s -> %s", original, path)
}

func LoadDocuments(root *ast.Document, opts LoadOptions) Snapshot {
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}

	snapshot := Snapshot{}
	if root == nil {
		return snapshot
	}
	snapshot.Sources = map[string]protocol.DocumentSource{
		filepath.Clean(root.URI): {
			URI:      root.URI,
			Path:     filepath.Clean(root.URI),
			Original: root.URI,
			Label:    filepath.Clean(root.URI),
		},
	}

	visited := map[string]bool{filepath.Clean(root.URI): true}
	queue := []*ast.Document{root}
	snapshot.Documents = append(snapshot.Documents, root)

	for len(queue) > 0 {
		doc := queue[0]
		queue = queue[1:]

		for _, ref := range documentImports(doc, opts.HomeDir) {
			if ref.IsFileImport {
				continue
			}

			path := filepath.Clean(ref.Resolved.Path)
			if visited[path] {
				continue
			}

			content, err := opts.ReadFile(path)
			if err != nil {
				snapshot.Issues = append(snapshot.Issues, LoadIssue{
					Import:  ref,
					Message: fmt.Sprintf("failed to read imported config %q: %v", ref.Raw, err),
				})
				continue
			}

			child, err := ast.Parse(path, content)
			if err != nil {
				snapshot.Issues = append(snapshot.Issues, LoadIssue{
					Import:  ref,
					Message: fmt.Sprintf("failed to parse imported config %q: %v", ref.Raw, err),
				})
				continue
			}

			visited[path] = true
			snapshot.Sources[path] = protocol.DocumentSource{
				URI:          path,
				Path:         path,
				Original:     ref.Raw,
				Label:        sourceLabel(path, ref.Raw),
				FromCache:    ref.Resolved.FromCache,
				ImportedFrom: &ref.Location,
			}
			snapshot.Documents = append(snapshot.Documents, child)
			queue = append(queue, child)
		}
	}

	return snapshot
}

func documentImports(doc *ast.Document, homeDir string) []ImportReference {
	baseDir := filepath.Dir(doc.URI)
	_, importNode, ok := doc.TopLevelSection("import")
	if !ok || importNode.Kind != yaml.SequenceNode {
		return nil
	}

	var refs []ImportReference
	for _, item := range ast.SequenceItems(importNode) {
		ref, ok := importReferenceFromNode(doc, baseDir, homeDir, item)
		if !ok {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

func importReferenceFromNode(doc *ast.Document, baseDir, homeDir string, item *yaml.Node) (ImportReference, bool) {
	ref := ImportReference{}
	rawNode := item

	switch item.Kind {
	case yaml.ScalarNode:
		ref.Raw = item.Value
	case yaml.MappingNode:
		_, srcNode, found := ast.LookupMappingValue(item, "src")
		if !found || srcNode.Kind != yaml.ScalarNode || srcNode.Value == "" {
			return ImportReference{}, false
		}
		rawNode = srcNode
		ref.Raw = srcNode.Value
		if _, _, found := ast.LookupMappingValue(item, "dest"); found {
			ref.IsFileImport = true
		}
	default:
		return ImportReference{}, false
	}

	ref.Location = doc.Location(rawNode)
	ref.Resolved = ResolveImport(baseDir, homeDir, ref.Raw)
	return ref, true
}
