package aitooling

import (
	"encoding/json"
	"testing"
)

func TestMustMarshalJSON_Success(t *testing.T) {
	input := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name",
			},
		},
	}

	result := MustMarshalJSON(input)

	// Verify it's valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Verify content is preserved
	if decoded["type"] != "object" {
		t.Errorf("Expected type=object, got %v", decoded["type"])
	}
}

func TestMustMarshalJSON_PanicsOnInvalidInput(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid input, but didn't panic")
		}
	}()

	// Channels cannot be marshaled to JSON
	invalidInput := make(chan int)
	MustMarshalJSON(invalidInput)
}

func TestEmptyJsonSchema(t *testing.T) {
	result := EmptyJsonSchema()

	// Verify it's valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Verify structure
	if decoded["type"] != "object" {
		t.Errorf("Expected type=object, got %v", decoded["type"])
	}

	properties, ok := decoded["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected properties to be a map")
	}
	if len(properties) != 0 {
		t.Errorf("Expected empty properties, got %d items", len(properties))
	}

	required, ok := decoded["required"].([]interface{})
	if !ok {
		t.Fatalf("Expected required to be an array")
	}
	if len(required) != 0 {
		t.Errorf("Expected empty required array, got %d items", len(required))
	}
}

func TestEmptyJsonSchema_CanBeUnmarshaledMultipleTimes(t *testing.T) {
	schema := EmptyJsonSchema()

	// First unmarshal
	var decoded1 map[string]interface{}
	if err := json.Unmarshal(schema, &decoded1); err != nil {
		t.Fatalf("First unmarshal failed: %v", err)
	}

	// Second unmarshal (verify we can reuse the schema)
	var decoded2 map[string]interface{}
	if err := json.Unmarshal(schema, &decoded2); err != nil {
		t.Fatalf("Second unmarshal failed: %v", err)
	}
}
