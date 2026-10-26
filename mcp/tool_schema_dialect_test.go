package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func roundTripSchema(t *testing.T, raw string) map[string]any {
	t.Helper()
	var schema ToolInputSchema
	require.NoError(t, json.Unmarshal([]byte(raw), &schema))
	data, err := json.Marshal(schema)
	require.NoError(t, err)

	var out map[string]any
	dec := json.NewDecoder(stringsReader(string(data)))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&out))
	return out
}

func TestToolSchema_DeclaredDraft07KeepsDefinitions(t *testing.T) {
	out := roundTripSchema(t, `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {"a": {"$ref": "#/definitions/item"}},
		"definitions": {"item": {"type": "string"}}
	}`)

	assert.Contains(t, out, "definitions")
	assert.NotContains(t, out, "$defs")
	assert.Equal(t, "#/definitions/item", out["properties"].(map[string]any)["a"].(map[string]any)["$ref"])
}

func TestToolSchema_KeepsBothDefinitionContainers(t *testing.T) {
	out := roundTripSchema(t, `{
		"type": "object",
		"properties": {
			"a": {"$ref": "#/$defs/new"},
			"b": {"$ref": "#/definitions/old"}
		},
		"$defs": {"new": {"type": "string"}},
		"definitions": {"old": {"type": "integer"}}
	}`)

	assert.Equal(t, map[string]any{"new": map[string]any{"type": "string"}}, out["$defs"])
	assert.Equal(t, map[string]any{"old": map[string]any{"type": "integer"}}, out["definitions"])
	props := out["properties"].(map[string]any)
	assert.Equal(t, "#/$defs/new", props["a"].(map[string]any)["$ref"])
	assert.Equal(t, "#/definitions/old", props["b"].(map[string]any)["$ref"])
}

func TestToolSchema_UndeclaredDefinitionsStillNormalized(t *testing.T) {
	out := roundTripSchema(t, `{
		"type": "object",
		"properties": {"a": {"$ref": "#/definitions/item"}},
		"definitions": {"item": {"type": "string"}}
	}`)

	assert.NotContains(t, out, "definitions")
	assert.Contains(t, out, "$defs")
	assert.Equal(t, "#/$defs/item", out["properties"].(map[string]any)["a"].(map[string]any)["$ref"])
}

func TestToolSchema_NormalizationSkipsInstanceData(t *testing.T) {
	out := roundTripSchema(t, `{
		"type": "object",
		"properties": {
			"a": {
				"$ref": "#/definitions/item",
				"default": {"$ref": "#/definitions/item"},
				"examples": [{"$ref": "#/definitions/item"}],
				"const": {"$ref": "#/definitions/item"},
				"x-vendor": {"$ref": "#/definitions/item"}
			}
		},
		"definitions": {"item": {"type": "string", "enum": [{"$ref": "#/definitions/item"}]}},
		"examples": [{"$ref": "#/definitions/item"}]
	}`)

	a := out["properties"].(map[string]any)["a"].(map[string]any)
	assert.Equal(t, "#/$defs/item", a["$ref"])
	assert.Equal(t, "#/definitions/item", a["default"].(map[string]any)["$ref"])
	assert.Equal(t, "#/definitions/item", a["examples"].([]any)[0].(map[string]any)["$ref"])
	assert.Equal(t, "#/definitions/item", a["const"].(map[string]any)["$ref"])
	assert.Equal(t, "#/definitions/item", a["x-vendor"].(map[string]any)["$ref"])
	item := out["$defs"].(map[string]any)["item"].(map[string]any)
	assert.Equal(t, "#/definitions/item", item["enum"].([]any)[0].(map[string]any)["$ref"])
	assert.Equal(t, "#/definitions/item", out["examples"].([]any)[0].(map[string]any)["$ref"])
}

func TestToolSchema_NormalizationFollowsSubschemaKeywords(t *testing.T) {
	out := roundTripSchema(t, `{
		"type": "object",
		"properties": {
			"list": {"type": "array", "items": {"$ref": "#/definitions/item"}},
			"pick": {"anyOf": [{"$ref": "#/definitions/item"}, {"type": "null"}]}
		},
		"additionalProperties": {"$ref": "#/definitions/item"},
		"not": {"$ref": "#/definitions/item"},
		"definitions": {"item": {"type": "object", "properties": {"self": {"$ref": "#/definitions/item"}}}}
	}`)

	props := out["properties"].(map[string]any)
	assert.Equal(t, "#/$defs/item", props["list"].(map[string]any)["items"].(map[string]any)["$ref"])
	assert.Equal(t, "#/$defs/item", props["pick"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)["$ref"])
	assert.Equal(t, "#/$defs/item", out["additionalProperties"].(map[string]any)["$ref"])
	assert.Equal(t, "#/$defs/item", out["not"].(map[string]any)["$ref"])
	self := out["$defs"].(map[string]any)["item"].(map[string]any)["properties"].(map[string]any)["self"]
	assert.Equal(t, "#/$defs/item", self.(map[string]any)["$ref"])
}

func TestToolSchema_AdditionalKeywordNumbersKeepPrecision(t *testing.T) {
	raw := `{"type":"object","maximum":9007199254740993,"x-big":1e400,"x-ratio":0.1}`
	var schema ToolInputSchema
	require.NoError(t, json.Unmarshal([]byte(raw), &schema))

	data, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"maximum":9007199254740993`)
	assert.Contains(t, string(data), `"x-big":1e400`)
	assert.Contains(t, string(data), `"x-ratio":0.1`)
}

func TestToolSchema_Draft04OmitsEmptyRequired(t *testing.T) {
	tests := []struct {
		name         string
		schema       string
		wantRequired bool
	}{
		{name: "draft-04", schema: `http://json-schema.org/draft-04/schema#`, wantRequired: false},
		{name: "draft-03", schema: `http://json-schema.org/draft-03/schema#`, wantRequired: false},
		{name: "draft-07", schema: `http://json-schema.org/draft-07/schema#`, wantRequired: true},
		{name: "2020-12", schema: `https://json-schema.org/draft/2020-12/schema`, wantRequired: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := roundTripSchema(t, `{"$schema":"`+tt.schema+`","type":"object","properties":{}}`)
			_, present := out["required"]
			assert.Equal(t, tt.wantRequired, present)
		})
	}

	out := roundTripSchema(t, `{"$schema":"http://json-schema.org/draft-04/schema#","type":"object","properties":{"a":{}},"required":["a"]}`)
	assert.Equal(t, []any{"a"}, out["required"])
}

func TestToolSchema_OutputSchemaKeepsDialect(t *testing.T) {
	var schema ToolOutputSchema
	require.NoError(t, json.Unmarshal([]byte(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {"a": {"$ref": "#/definitions/item"}},
		"definitions": {"item": {"type": "string"}}
	}`), &schema))
	data, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"definitions":{"item":{"type":"string"}}`)
	assert.Contains(t, string(data), `"$ref":"#/definitions/item"`)
	assert.NotContains(t, string(data), `"$defs"`)
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
