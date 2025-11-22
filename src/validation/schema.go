package validation

import (
	"bytes"
	"encoding/json"
	"fmt"

	"temporal-poc/src/core/domain"
	"temporal-poc/src/register"

	"github.com/invopop/jsonschema"
	jsonschemav5 "github.com/santhosh-tekuri/jsonschema/v5"
)

// ConvertStructToJSONSchema converts a Go struct to JSON Schema format
func ConvertStructToJSONSchema(schemaStruct interface{}) ([]byte, error) {
	if schemaStruct == nil {
		return nil, fmt.Errorf("schema struct is nil")
	}

	// Use invopop/jsonschema to convert struct to JSON Schema
	reflector := jsonschema.Reflector{}
	reflector.RequiredFromJSONSchemaTags = true // Use jsonschema tags for required fields
	schema := reflector.Reflect(schemaStruct)

	// Convert to JSON bytes
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
	}

	return schemaBytes, nil
}

// ValidateStepSchema validates a step's input against the node's schema
func ValidateStepSchema(nodeName string, stepInput map[string]interface{}) error {
	reg := register.GetInstance()

	nodeInfo, exists := reg.GetNodeInfo(nodeName)
	if !exists {
		return nil // Skip if node doesn't exist (this will be caught by other validation)
	}

	// If node has no schema, no validation needed
	if nodeInfo.Schema == nil || nodeInfo.Schema.SchemaStruct == nil {
		return nil
	}

	// If no input provided, use empty map
	if stepInput == nil {
		stepInput = make(map[string]interface{})
	}

	// Convert struct to JSON Schema
	jsonSchemaBytes, err := ConvertStructToJSONSchema(nodeInfo.Schema.SchemaStruct)
	if err != nil {
		return fmt.Errorf("failed to convert schema struct to JSON Schema for node '%s': %w", nodeName, err)
	}

	// Compile JSON Schema
	compiler := jsonschemav5.NewCompiler()
	schemaID := fmt.Sprintf("schema://%s", nodeName)
	compiler.AddResource(schemaID, bytes.NewReader(jsonSchemaBytes))
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		return fmt.Errorf("failed to compile schema for node '%s': %w", nodeName, err)
	}

	// Convert step input to JSON for validation
	stepInputJSON, err := json.Marshal(stepInput)
	if err != nil {
		return fmt.Errorf("failed to marshal input data for node '%s': %w", nodeName, err)
	}

	var stepInputUnmarshaled interface{}
	if err := json.Unmarshal(stepInputJSON, &stepInputUnmarshaled); err != nil {
		return fmt.Errorf("failed to unmarshal input data for node '%s': %w", nodeName, err)
	}

	// Validate data against schema
	if err := schema.Validate(stepInputUnmarshaled); err != nil {
		return fmt.Errorf("schema validation failed for node '%s': %w", nodeName, err)
	}

	return nil
}

// ValidateSchemaData validates that all required schema fields are provided for all nodes in the workflow
// It converts Go struct schemas to JSON Schema and validates the data
// DEPRECATED: Use ValidateStepInput instead, which validates input directly from StepConfig
func ValidateSchemaData(config register.WorkflowDefinition, schemaData domain.SchemaData) error {
	reg := register.GetInstance()

	// Collect all unique node names used in the workflow
	nodeNames := make(map[string]bool)
	for _, stepDef := range config.Steps {
		if stepDef.Node != "" {
			nodeNames[stepDef.Node] = true
		}
	}

	// Validate schema for each node
	for nodeName := range nodeNames {
		nodeInfo, exists := reg.GetNodeInfo(nodeName)
		if !exists {
			continue // Skip if node doesn't exist (this will be caught by other validation)
		}

		// If node has a schema struct, validate the provided data
		if nodeInfo.Schema != nil && nodeInfo.Schema.SchemaStruct != nil {
			nodeData, hasData := schemaData[nodeName]
			if !hasData {
				nodeData = make(map[string]interface{})
			}

			// Convert struct to JSON Schema
			jsonSchemaBytes, err := ConvertStructToJSONSchema(nodeInfo.Schema.SchemaStruct)
			if err != nil {
				return fmt.Errorf("failed to convert schema struct to JSON Schema for node '%s': %w", nodeName, err)
			}

			// Compile JSON Schema
			compiler := jsonschemav5.NewCompiler()
			schemaID := fmt.Sprintf("schema://%s", nodeName)
			compiler.AddResource(schemaID, bytes.NewReader(jsonSchemaBytes))
			schema, err := compiler.Compile(schemaID)
			if err != nil {
				return fmt.Errorf("failed to compile schema for node '%s': %w", nodeName, err)
			}

			// Convert node data to JSON for validation
			nodeDataJSON, err := json.Marshal(nodeData)
			if err != nil {
				return fmt.Errorf("failed to marshal data for node '%s': %w", nodeName, err)
			}

			var nodeDataUnmarshaled interface{}
			if err := json.Unmarshal(nodeDataJSON, &nodeDataUnmarshaled); err != nil {
				return fmt.Errorf("failed to unmarshal data for node '%s': %w", nodeName, err)
			}

			// Validate data against schema
			if err := schema.Validate(nodeDataUnmarshaled); err != nil {
				return fmt.Errorf("schema validation failed for node '%s': %w", nodeName, err)
			}
		}
	}

	return nil
}
