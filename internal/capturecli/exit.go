package capturecli

import (
	"fmt"
	"io"
)

// ExitCode keeps native hook success, failure and recovered panic silent.
// Ordinary CLI errors retain their content-free diagnostic behavior.
func ExitCode(args []string, execute func() error, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "hook" {
		return hookExitCode(execute)
	}
	if err := execute(); err != nil {
		fmt.Fprintln(diagnostic, err)
		return 1
	}
	return 0
}

// A recovered handler panic must not become Go's exit 2, which Crush treats
// as denial. Runtime fatal errors and unguarded goroutines remain outside it.
func hookExitCode(execute func() error) (status int) {
	status = 1
	defer func() { _ = recover() }()
	if execute() == nil {
		return 0
	}
	return status
}
