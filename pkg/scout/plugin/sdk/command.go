package sdk

import "context"

// CommandHandler handles a CLI command forwarded from Scout.
type CommandHandler interface {
	Execute(ctx context.Context, params CommandParams) (*CommandResult, error)
}

// CommandHandlerFunc adapts a function to CommandHandler.
type CommandHandlerFunc func(ctx context.Context, params CommandParams) (*CommandResult, error)

func (f CommandHandlerFunc) Execute(ctx context.Context, params CommandParams) (*CommandResult, error) {
	return f(ctx, params)
}

// CommandParams are the parameters for a command/execute request.
type CommandParams struct {
	Command        string         `json:"command"`
	Args           []string       `json:"args"`
	Flags          map[string]any `json:"flags,omitempty"`
	BrowserContext *BrowserContext `json:"browser_context,omitempty"`
}

// BrowserContext provides browser connection details for commands that require a browser.
type BrowserContext struct {
	CDPEndpoint string `json:"cdp_endpoint"`
	SessionDir  string `json:"session_dir"`
	SessionID   string `json:"session_id"`
}

// CommandResult is the result of a command execution.
type CommandResult struct {
	Output      string `json:"output"`
	ExitCode    int    `json:"exit_code"`
	ContentType string `json:"content_type,omitempty"`
}

// CommandOutput creates a successful CommandResult with the given output text.
func CommandOutput(output string) *CommandResult {
	return &CommandResult{
		Output:   output,
		ExitCode: 0,
	}
}

// CommandError creates a failed CommandResult with the given error message and exit code.
func CommandError(msg string, exitCode int) *CommandResult {
	return &CommandResult{
		Output:   msg,
		ExitCode: exitCode,
	}
}

// CompletionHandler provides shell completion suggestions for a command.
type CompletionHandler interface {
	Complete(ctx context.Context, params CompletionParams) ([]string, error)
}

// CompletionParams are the parameters for a command/complete request.
type CompletionParams struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	ToComp  string   `json:"to_complete"`
}
