package validation

import (
	"fmt"

	"temporal-poc/src/register"
)

// ValidateWorkflowDefinition validates the workflow definition for circular dependencies
// It checks all possible paths from the start step to detect cycles
// This should be called before executing the workflow
func ValidateWorkflowDefinition(definition register.WorkflowDefinition, startStep string) error {
	// Check if start step exists
	if _, exists := definition[startStep]; !exists {
		return fmt.Errorf("start step not found in definition: %s", startStep)
	}

	// Use DFS to detect cycles
	// visited tracks all visited nodes in the current path
	// globallyVisited tracks all nodes we've already checked (optimization)
	visited := make(map[string]bool)
	globallyVisited := make(map[string]bool)

	var dfs func(step string) error
	dfs = func(step string) error {
		// If we've already fully checked this node, skip it
		if globallyVisited[step] {
			return nil
		}

		// Check for cycle in current path
		if visited[step] {
			return fmt.Errorf("circular workflow definition detected at step: %s", step)
		}

		// Get step definition
		stepDef, exists := definition[step]
		if !exists {
			return fmt.Errorf("step definition not found: %s", step)
		}

		// Mark as visited in current path
		visited[step] = true
		defer func() {
			// Remove from current path when backtracking
			delete(visited, step)
			// Mark as globally visited after checking all paths from this node
			globallyVisited[step] = true
		}()

		// Check all possible next steps (from conditions and go_to)
		nextSteps := make(map[string]bool)

		// Add go_to step if exists
		if stepDef.GoTo != "" {
			nextSteps[stepDef.GoTo] = true
		}

		// Add all condition steps
		if stepDef.Conditions != nil {
			if stepDef.Conditions.Success != "" {
				nextSteps[stepDef.Conditions.Success] = true
			}
			if stepDef.Conditions.Timeout != "" {
				nextSteps[stepDef.Conditions.Timeout] = true
			}
		}

		// Recursively check all next steps
		for nextStep := range nextSteps {
			if err := dfs(nextStep); err != nil {
				return err
			}
		}

		return nil
	}

	// Start DFS from the start step
	return dfs(startStep)
}
