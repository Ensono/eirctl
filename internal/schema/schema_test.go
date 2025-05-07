package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/internal/schema"
	"github.com/invopop/jsonschema"
)

func Test_StringSlice_JSONSchema(t *testing.T) {
	type testSchema struct {
		StrSliceList schema.StringSlice `jsonschema:"oneof_type=string;array"`
		str          string             `jsonschema:"oneof_required=task"`
	}

	r := new(jsonschema.Reflector)
	s := r.Reflect(&testSchema{})
	// use 2 spaces for indentation
	out, err := json.MarshalIndent(s, "", `  `)
	if err != nil {
		t.Fatalf("failed to parse: %s", err)
	}
	want := "\"$defs\": {\n    \"StringSlice\": {\n      \"oneOf\": [\n        {\n          \"type\": \"string\"\n        },\n        {\n          \"items\": {\n            \"type\": \"string\"\n          },\n          \"type\": \"array\"\n        }\n      ]\n    }"
	if !strings.Contains(string(out), want) {
		t.Fatalf("not parsed properly got: %s, wanted to include %s", string(out), want)
	}
}
