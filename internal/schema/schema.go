package schema

import (
	"fmt"

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
