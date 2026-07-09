package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes a command, streaming its output to the process stdout/stderr.
// Used for long or interactive-ish operations (package installs) where the
// user benefits from live output.
func Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// RunCaptured executes a command and, on failure, includes its combined
// output in the returned error, for short commands whose diagnostics must
// reach the user (e.g. config validation) instead of only the process log.
func RunCaptured(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// Output runs a command and returns its trimmed combined stdout.
func Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// LookPath reports whether an executable is available on PATH.
func LookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// IsRoot reports whether the process runs as uid 0.
func IsRoot() bool { return os.Geteuid() == 0 }
