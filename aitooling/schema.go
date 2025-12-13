package aitooling

import "encoding/json"

// Helper to marshal params to JSON
func MustMarshalJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func EmptyJsonSchema() json.RawMessage {
	params := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
	b, _ := json.Marshal(params)
	return b
}
