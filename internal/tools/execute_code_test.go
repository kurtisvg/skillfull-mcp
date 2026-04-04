package tools

import (
	"context"
	"strings"
	"testing"

	"skillful-mcp/internal/mcpserver"

	monty "github.com/ewhauser/gomonty"
)

func TestExecuteCodeDescriptionRefersToUseSkill(t *testing.T) {
	t.Parallel()
	if !strings.Contains(executeCodeDescription, "use_skill") {
		t.Error("description should refer to use_skill for tool discovery")
	}
	if !strings.Contains(executeCodeDescription, "tool_name") {
		t.Error("description should show tools are called by name")
	}
	if !strings.Contains(executeCodeDescription, "resources") {
		t.Error("description should mention resources")
	}
}

func TestExecuteCodeBasicMath(t *testing.T) {
	t.Parallel()
	runner, err := monty.New("40 + 2", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(t.Context(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "42" {
		t.Errorf("result = %q, want '42'", value.String())
	}
}

func TestExecuteCodeStringExpression(t *testing.T) {
	t.Parallel()
	runner, err := monty.New("'hello' + ' ' + 'world'", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(t.Context(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "hello world" {
		t.Errorf("result = %q, want 'hello world'", value.String())
	}
}

func TestExecuteCodeSyntaxError(t *testing.T) {
	t.Parallel()
	_, err := monty.New("def (invalid syntax", monty.CompileOptions{ScriptName: "script.py"})
	if err == nil {
		t.Fatal("expected compile error for invalid syntax")
	}
}

// --- validateMontyValue tests ---

func TestValidateMontyValueStringMatch(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.String("hello"), mcpserver.ParamInfo{Name: "x", Types: []string{"string"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueIntegerMatch(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Int(42), mcpserver.ParamInfo{Name: "x", Types: []string{"integer"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueNumberAcceptsInt(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Int(42), mcpserver.ParamInfo{Name: "x", Types: []string{"number"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueNumberAcceptsFloat(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Float(3.14), mcpserver.ParamInfo{Name: "x", Types: []string{"number"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueBooleanMatch(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Bool(true), mcpserver.ParamInfo{Name: "x", Types: []string{"boolean"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueArrayMatch(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.List(monty.Int(1)), mcpserver.ParamInfo{Name: "x", Types: []string{"array"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueObjectMatch(t *testing.T) {
	t.Parallel()
	dict := monty.DictValue(monty.Dict{{Key: monty.String("k"), Value: monty.String("v")}})
	err := validateMontyValue(dict, mcpserver.ParamInfo{Name: "x", Types: []string{"object"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueTypeMismatch(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Int(42), mcpserver.ParamInfo{Name: "sql", Types: []string{"string"}})
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
	if !strings.Contains(err.Error(), "sql") {
		t.Errorf("error should mention parameter name, got: %v", err)
	}
}

func TestValidateMontyValueNullableString(t *testing.T) {
	t.Parallel()
	// None should pass for ["string", "null"].
	err := validateMontyValue(monty.None(), mcpserver.ParamInfo{Name: "x", Types: []string{"string", "null"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// String should also pass.
	err = validateMontyValue(monty.String("hi"), mcpserver.ParamInfo{Name: "x", Types: []string{"string", "null"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateMontyValueNoTypes(t *testing.T) {
	t.Parallel()
	// Nil types should skip validation.
	err := validateMontyValue(monty.Int(42), mcpserver.ParamInfo{Name: "x", Types: nil})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Integration: type validation via Monty runner ---

func TestExecuteCodeTypeValidation(t *testing.T) {
	t.Parallel()
	// Register a function with a string parameter schema, then call it with an int.
	params := []mcpserver.ParamInfo{{Name: "name", Types: []string{"string"}}}
	paramByName := map[string]mcpserver.ParamInfo{"name": params[0]}

	fn := func(fnCtx context.Context, call monty.Call) (monty.Result, error) {
		args := make(map[string]any)
		for i, val := range call.Args {
			if i < len(params) {
				if err := validateMontyValue(val, params[i]); err != nil {
					msg := err.Error()
					return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
				}
				args[params[i].Name] = montyValueToAny(val)
			}
		}
		for _, pair := range call.Kwargs {
			key, ok := pair.Key.Raw().(string)
			if !ok {
				continue
			}
			if pi, ok := paramByName[key]; ok {
				if err := validateMontyValue(pair.Value, pi); err != nil {
					msg := err.Error()
					return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
				}
			}
			args[key] = montyValueToAny(pair.Value)
		}
		return monty.Return(monty.String("ok")), nil
	}

	// Valid call: string argument.
	t.Run("valid_string_arg", func(t *testing.T) {
		runner, err := monty.New(`greet("world")`, monty.CompileOptions{ScriptName: "test.py"})
		if err != nil {
			t.Fatal(err)
		}
		value, err := runner.Run(t.Context(), monty.RunOptions{
			Functions: map[string]monty.ExternalFunction{"greet": fn},
		})
		if err != nil {
			t.Fatalf("runtime error: %v", err)
		}
		if value.String() != "ok" {
			t.Errorf("result = %q, want 'ok'", value.String())
		}
	})

	// Invalid call: int argument where string expected.
	t.Run("invalid_int_for_string", func(t *testing.T) {
		runner, err := monty.New(`greet(42)`, monty.CompileOptions{ScriptName: "test.py"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.Run(t.Context(), monty.RunOptions{
			Functions: map[string]monty.ExternalFunction{"greet": fn},
		})
		if err == nil {
			t.Fatal("expected runtime error for type mismatch")
		}
		if !strings.Contains(err.Error(), "TypeError") {
			t.Errorf("expected TypeError, got: %v", err)
		}
	})
}
