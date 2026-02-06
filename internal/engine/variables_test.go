package engine

import (
	"testing"
)

// TestVariables tests the SetVar and GetVar functionality via variable resolution.
func TestVariables(t *testing.T) {

	execCtx := NewExecutionContext("test")

	// Simulate set_var action
	execCtx.SetVar("counter", 42)
	execCtx.SetVar("name", "Test User")

	// Test that variables are stored correctly
	counter := execCtx.GetVar("counter")
	if counter == nil {
		t.Fatal("Variable 'counter' was not set")
	}

	// Check value
	var counterValue int
	switch v := counter.(type) {
	case int:
		counterValue = v
	case float64:
		counterValue = int(v)
	default:
		t.Fatalf("Unexpected counter type: %T", counter)
	}

	if counterValue != 42 {
		t.Errorf("Expected counter=42, got %d", counterValue)
	}

	// Test variable resolution in config
	config := map[string]interface{}{
		"message": "Hello, {{ $vars.name }}! Your counter is {{ $vars.counter }}",
		"value":   "{{ $vars.counter }}",
	}

	resolved, err := ResolveVariables(config, execCtx)
	if err != nil {
		t.Fatalf("Variable resolution failed: %v", err)
	}

	resolvedMap, ok := resolved.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", resolved)
	}

	expectedMessage := "Hello, Test User! Your counter is 42"
	if resolvedMap["message"] != expectedMessage {
		t.Errorf("Expected message %q, got %q", expectedMessage, resolvedMap["message"])
	}

	if resolvedMap["value"] != 42 {
		t.Errorf("Expected value 42, got %v", resolvedMap["value"])
	}

	t.Logf("Variable test passed: counter=%v, name=%v", counter, execCtx.GetVar("name"))
}

// TestVariableResolution tests the {{ $vars.* }} syntax in variable resolver.
func TestVariableResolution(t *testing.T) {
	execCtx := NewExecutionContext("test")
	execCtx.SetVar("name", "Zaraza")
	execCtx.SetVar("age", 25)

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "Simple variable",
			input:    "{{ $vars.name }}",
			expected: "Zaraza",
		},
		{
			name:     "Variable in string",
			input:    "Hello, {{ $vars.name }}!",
			expected: "Hello, Zaraza!",
		},
		{
			name:     "Numeric variable",
			input:    "{{ $vars.age }}",
			expected: 25,
		},
		{
			name: "Variable in map",
			input: map[string]interface{}{
				"greeting": "Hello, {{ $vars.name }}",
				"age":      "{{ $vars.age }}",
			},
			expected: map[string]interface{}{
				"greeting": "Hello, Zaraza",
				"age":      25,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveVariables(tt.input, execCtx)
			if err != nil {
				t.Fatalf("ResolveVariables failed: %v", err)
			}

			// Compare results
			switch expected := tt.expected.(type) {
			case string:
				if result != expected {
					t.Errorf("Expected %q, got %q", expected, result)
				}
			case int:
				if result != expected {
					t.Errorf("Expected %d, got %v", expected, result)
				}
			case map[string]interface{}:
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected map, got %T", result)
				}
				for key, expectedVal := range expected {
					if resultMap[key] != expectedVal {
						t.Errorf("Key %s: expected %v, got %v", key, expectedVal, resultMap[key])
					}
				}
			}
		})
	}
}

// TestVariableNotFound tests error handling for undefined variables.
func TestVariableNotFound(t *testing.T) {
	execCtx := NewExecutionContext("test")

	input := "{{ $vars.undefined }}"
	_, err := ResolveVariables(input, execCtx)
	if err == nil {
		t.Fatal("Expected error for undefined variable, got nil")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}

	t.Logf("Correctly caught error: %v", err)
}
