package helpers

import (
	"encoding/json"
	"fmt"
	"reflect"

	"temporal-poc/src/core/domain"
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

// ValidateSchemaUsingNodeSchema validates a schema map against a NodeSchema using the same method as UnmarshalSchema
// It uses reflection to create an instance of the schema struct type and unmarshals into it
// Returns an error if validation fails
func ValidateSchemaUsingNodeSchema(nodeSchema *domain.NodeSchema, schema map[string]interface{}) error {
	if nodeSchema == nil || nodeSchema.SchemaStruct == nil {
		// No schema to validate against
		return nil
	}

	if schema == nil {
		// Empty schema is valid
		return nil
	}

	// Get the type of the schema struct
	schemaType := reflect.TypeOf(nodeSchema.SchemaStruct)
	if schemaType.Kind() == reflect.Ptr {
		schemaType = schemaType.Elem()
	}

	// Create a new instance of the schema struct type
	schemaValue := reflect.New(schemaType).Interface()

	// Marshal the schema map to JSON
	jsonData, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Unmarshal JSON into the schema struct instance
	err = json.Unmarshal(jsonData, schemaValue)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}
