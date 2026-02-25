package schema

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

// Generic functions go here

// StringSlice is a []string that can unmarshal from either
// a YAML string or a YAML sequence of strings.
type StringSlice []string

// JSONSchema implements jsonschema.ExtSchema
func (StringSlice) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "string"},
			},
		},
	}
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *StringSlice) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// Single string — decode into a Go string, then wrap
		var single string
		if err := value.Decode(&single); err != nil {
			return err
		}
		*s = StringSlice{single}
	case yaml.SequenceNode:
		// Sequence of strings — decode directly into a []string
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*s = list
	default:
		return fmt.Errorf("cannot unmarshal %v into StringSlice", value.Kind)
	}
	return nil
}

// ImportEntry represents an entry in the unified import list.
// Supports both plain string form (backward compatible config import)
// and object form with src, hash, and dest fields.
//
// Plain strings are treated as config imports (YAML merged into config).
// Objects with a dest field are treated as file imports (written to disk).
// The dest field is required for file imports and specifies the path relative to the project root.
// Objects without dest are treated as config imports with optional hash verification.
type ImportEntry struct {
	// Src of the config or file
	Src string `yaml:"src" json:"src"`
	// Hash is the sha256:$SUM of the file
	// currently only sha256 is supported but others might be supported in the future
	Hash string `yaml:"hash,omitempty" json:"hash,omitempty"`
	// Dest is the destination on local disk where the file can be picked up from
	Dest string `yaml:"dest,omitempty" json:"dest,omitempty"`
}

type HashType string

const (
	Sha256 HashType = "sha256"
)

// IsFileImport returns true if this entry should be handled as a file import
// (written to disk rather than parsed as config). An entry is a file import
// when dest is explicitly set.
func (ie *ImportEntry) IsFileImport() bool {
	return len(ie.Dest) > 0
}

// ParsedHash struct holds the `algorithm:hex_digest` combination
type ParsedHash struct {
	Typ HashType
	Val string
}

// ErrUnsupportedHashFormat occurs when an unsupported hash algorithm is specified
var ErrUnsupportedHashFormat = errors.New("unsupported hash format, must be in 'algorithm:hex_digest' format")

// ErrUnsupportedHashAlgorithm ensures the hash algorithm is supported
var ErrUnsupportedHashAlgorithm = errors.New("unsupported hash, must be one of ['sha256']")

// ErrSrcEmpty ensures an empty string for source throws an error
var ErrSrcEmpty = errors.New("import entry source must not be empty")

// HasHash checks whether a hash verification has been provided and should be performed
func (ie *ImportEntry) HasHash() (bool, ParsedHash, error) {
	if len(ie.Hash) > 0 {
		parts := strings.SplitN(ie.Hash, ":", 2)
		if len(parts) != 2 {
			return false, ParsedHash{}, fmt.Errorf("%w, got %q", ErrUnsupportedHashFormat, ie.Hash)
		}
		if slices.Contains([]HashType{Sha256}, HashType(parts[0])) {
			return true, ParsedHash{HashType(parts[0]), parts[1]}, nil
		}
		return false, ParsedHash{}, fmt.Errorf("%w, got %s", ErrUnsupportedHashAlgorithm, parts[0])
	}
	return false, ParsedHash{}, nil
}

// UnmarshalYAML supports both string and object forms in the import list.
func (ie *ImportEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		// Backwards compat: plain string is assigned as Src
		if len(value.Value) > 0 {
			ie.Src = value.Value
			return nil
		}
		return fmt.Errorf("%w", ErrSrcEmpty)
	}
	if value.Kind == yaml.MappingNode {
		// casting the mapping node into the `ImportEntry` type
		// without this the `value.Decode((*ImportEntry)(ie))`
		// would cause an infinite loop and eventually SEGV due to stack overflow
		type plain ImportEntry
		var tmp plain
		if err := value.Decode(&tmp); err != nil {
			return err
		}
		if len(tmp.Src) > 0 {
			*ie = ImportEntry(tmp)
			return nil
		}
		return fmt.Errorf("%w", ErrSrcEmpty)
	}
	return fmt.Errorf("import entry must be a string or object, got %v", value.Kind)
}
