package config_test

import (
	"encoding/json"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/invopop/jsonschema"
)

func Test_JSONSchema(t *testing.T) {
	r := new(jsonschema.Reflector)
	if err := r.AddGoComments("github.com/Ensono/eirctl", "./"); err != nil {
		t.Fatal(err.Error())
	}
	s := r.Reflect(&config.ConfigDefinition{})
	// use 2 spaces for indentation
	out, err := json.MarshalIndent(s, "", `  `)
	if err != nil {
		t.Fatalf("failed to parse: %s", err)
	}
	if len(out) < 1 {
		t.Fatal("not written any bytes into jsonschema")
	}
}
