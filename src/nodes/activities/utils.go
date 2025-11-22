package activities

import (
	"encoding/json"
)

// UnmarshalSchema unmarshals the schema map into the specified struct type using generics
// Returns the struct instance and an error if unmarshaling fails
// Usage: schema, err := UnmarshalSchema[MySchemaType](activityCtx.Schema)
func UnmarshalSchema[T any](schema map[string]interface{}) (T, error) {
	var result T
	if schema == nil {
		return result, nil
	}

	// Marshal map to JSON
	jsonData, err := json.Marshal(schema)
	if err != nil {
		return result, err
	}

	// Unmarshal JSON into struct type
	err = json.Unmarshal(jsonData, &result)
	return result, err
}
