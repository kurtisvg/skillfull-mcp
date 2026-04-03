package tools

import (
	"context"
	"strings"
	"testing"

	monty "github.com/ewhauser/gomonty"
)

func TestExecuteCodeDescriptionRefersToUseSkill(t *testing.T) {
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
	runner, err := monty.New("40 + 2", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(context.Background(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "42" {
		t.Errorf("result = %q, want '42'", value.String())
	}
}

func TestExecuteCodeStringExpression(t *testing.T) {
	runner, err := monty.New("'hello' + ' ' + 'world'", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(context.Background(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "hello world" {
		t.Errorf("result = %q, want 'hello world'", value.String())
	}
}

func TestExecuteCodeSyntaxError(t *testing.T) {
	_, err := monty.New("def (invalid syntax", monty.CompileOptions{ScriptName: "script.py"})
	if err == nil {
		t.Fatal("expected compile error for invalid syntax")
	}
}
