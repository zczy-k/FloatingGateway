// Package exec provides a unified interface for executing external commands.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout is the default command execution timeout.
const DefaultTimeout = 30 * time.Second

// Result holds the result of a command execution.
type Result struct {
	Command  string
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Err      error
}

// Success returns true if the command exited with code 0.
func (r *Result) Success() bool {
	return r.ExitCode == 0 && r.Err == nil
}

// Combined returns stdout and stderr combined.
func (r *Result) Combined() string {
	return strings.TrimSpace(r.Stdout + "\n" + r.Stderr)
}

// Executor is the interface for command execution.
type Executor interface {
	Run(ctx context.Context, name string, args ...string) *Result
	RunWithTimeout(name string, timeout time.Duration, args ...string) *Result
}

// RealExecutor executes commands on the actual system.
type RealExecutor struct{}

// NewExecutor creates a new real executor.
func NewExecutor() *RealExecutor {
	return &RealExecutor{}
}

// Run executes a command with the given context.
func (e *RealExecutor) Run(ctx context.Context, name string, args ...string) *Result {
	start := time.Now()
	result := &Result{
		Command: name,
		Args:    args,
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		result.Err = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}

	return result
}

// RunWithTimeout executes a command with the specified timeout.
func (e *RealExecutor) RunWithTimeout(name string, timeout time.Duration, args ...string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return e.Run(ctx, name, args...)
}

// RunDefault executes a command with the default timeout.
func (e *RealExecutor) RunDefault(name string, args ...string) *Result {
	return e.RunWithTimeout(name, DefaultTimeout, args...)
}

// MockExecutor is a mock executor for testing.
type MockExecutor struct {
	Results map[string]*Result
	Default *Result
	Calls   []MockCall
}

// MockCall records a call made to the mock executor.
type MockCall struct {
	Command string
	Args    []string
}

// NewMockExecutor creates a new mock executor.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Results: make(map[string]*Result),
		Default: &Result{ExitCode: 0},
		Calls:   make([]MockCall, 0),
	}
}

// SetResult sets the result for a specific command.
func (m *MockExecutor) SetResult(key string, result *Result) {
	m.Results[key] = result
}

// Run executes and returns the mocked result.
func (m *MockExecutor) Run(ctx context.Context, name string, args ...string) *Result {
	m.Calls = append(m.Calls, MockCall{Command: name, Args: args})
	
	// Try exact match first
	key := name + " " + strings.Join(args, " ")
	if result, ok := m.Results[key]; ok {
		return result
	}
	
	// Try command name only
	if result, ok := m.Results[name]; ok {
		return result
	}
	
	return m.Default
}

// RunWithTimeout executes and returns the mocked result.
func (m *MockExecutor) RunWithTimeout(name string, timeout time.Duration, args ...string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m.Run(ctx, name, args...)
}

// Global executor instance (can be replaced for testing)
var defaultExecutor Executor = NewExecutor()

// SetDefaultExecutor sets the global executor (for testing).
func SetDefaultExecutor(e Executor) {
	defaultExecutor = e
}

// GetDefaultExecutor returns the global executor.
func GetDefaultExecutor() Executor {
	return defaultExecutor
}

// Run executes a command with the given context using the global executor.
func Run(ctx context.Context, name string, args ...string) *Result {
	return defaultExecutor.Run(ctx, name, args...)
}

// RunStdout executes a command and returns its stdout as string.
func RunStdout(name string, args ...string) (string, error) {
	// Split name if it contains spaces (for simple cases like "ls -la")
	var cmdName string
	var cmdArgs []string
	
	if strings.Contains(name, " ") && len(args) == 0 {
		parts := strings.Fields(name)
		cmdName = parts[0]
		cmdArgs = parts[1:]
	} else {
		cmdName = name
		cmdArgs = args
	}

	result := defaultExecutor.RunWithTimeout(cmdName, DefaultTimeout, cmdArgs...)
	if !result.Success() {
		return "", fmt.Errorf("command failed: %s (exit code %d): %s", cmdName, result.ExitCode, result.Combined())
	}
	return strings.TrimSpace(result.Stdout), nil
}

// RunWithTimeout executes a command with timeout using the global executor.
func RunWithTimeout(name string, timeout time.Duration, args ...string) *Result {
	return defaultExecutor.RunWithTimeout(name, timeout, args...)
}

// Which finds the path to an executable.
func Which(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("command %q not found: %w", name, err)
	}
	return path, nil
}

// CommandExists checks if a command exists.
func CommandExists(name string) bool {
	_, err := Which(name)
	return err == nil
}
