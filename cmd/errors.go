package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/EstebanForge/operator/pkg/tmux"
)

// Exit codes conforming to SPECS.md Section 5
const (
	ExitSuccess    = 0
	ExitDaemon     = 1
	ExitNotFound   = 2
	ExitConflict   = 3
	ExitValidation = 4
)

// ErrorResponse represents the standardized JSON error contract.
type ErrorResponse struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HandleError formats and prints the error and terminates execution with the appropriate exit code.
func HandleError(err error) {
	if err == nil {
		os.Exit(ExitSuccess)
	}

	code := ResolveExitCode(err)
	msg := err.Error()

	if errors.Is(err, tmux.ErrTmuxMissing) {
		info := tmux.DetectInstallInfo()
		if jsonFlag {
			resp := ErrorResponse{
				Status:  "error",
				Code:    code,
				Message: fmt.Sprintf("tmux binary not found in PATH (install via '%s')", info.Command),
			}
			data, mErr := json.MarshalIndent(resp, "", "  ")
			if mErr != nil {
				fmt.Fprintf(os.Stderr, "{\"status\":\"error\",\"code\":%d,\"message\":%q}\n", code, resp.Message)
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", string(data))
			}
		} else {
			fmt.Fprintln(os.Stderr, tmux.MissingTmuxPrompt())
		}
		os.Exit(code)
	}

	if jsonFlag {
		resp := ErrorResponse{
			Status:  "error",
			Code:    code,
			Message: msg,
		}
		data, mErr := json.MarshalIndent(resp, "", "  ")
		if mErr != nil {
			fmt.Fprintf(os.Stderr, "{\"status\":\"error\",\"code\":%d,\"message\":%q}\n", code, msg)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", string(data))
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}

	os.Exit(code)
}

// ResolveExitCode maps known errors to their corresponding exit codes.
func ResolveExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, tmux.ErrNotFound) {
		return ExitNotFound
	}
	if errors.Is(err, tmux.ErrConflict) {
		return ExitConflict
	}
	if errors.Is(err, tmux.ErrValidation) || errors.Is(err, tmux.ErrNotATTY) {
		return ExitValidation
	}
	return ExitDaemon
}

// OutputJSON prints a value as indented JSON to stdout.
func OutputJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
