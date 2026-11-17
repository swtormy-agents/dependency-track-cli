package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const writePrompt = "This command changes Dependency-Track. Type 'write' to confirm: "

// requireHumanWrite gates mutating commands: --write flag and an interactive TTY.
// Call it before any other work; call confirmWrite once the request is built,
// right before it is sent.
func (a *app) requireHumanWrite(cmd *cobra.Command, what string) error {
	writeFlag := false
	if cmd != nil {
		f := cmd.Flags().Lookup("write")
		if f != nil {
			writeFlag = f.Changed && f.Value.String() == "true"
		}
	}
	if !writeFlag {
		return authErr(fmt.Errorf("%s is read-only for agents; a human must run this command with --write in an interactive terminal", what))
	}
	if !isTerminal(a.in) {
		return authErr(fmt.Errorf("%s with --write requires an interactive terminal; agents cannot perform writes", what))
	}
	return nil
}

// confirmWrite shows the exact request that is about to be sent and asks the
// human to type "write". body may be nil; extra lines (e.g. "file: bom.json")
// are printed after the request line.
func (a *app) confirmWrite(method, url string, body any, extra ...string) error {
	fmt.Fprintf(a.errOut, "About to send: %s %s\n", method, url)
	for _, line := range extra {
		fmt.Fprintf(a.errOut, "  %s\n", line)
	}
	if body != nil {
		b, err := json.MarshalIndent(body, "  ", "  ")
		if err != nil {
			return runtimeErr(err)
		}
		fmt.Fprintf(a.errOut, "  body:\n  %s\n", b)
	}
	fmt.Fprint(a.errOut, writePrompt)
	var ans string
	if _, err := fmt.Fscanln(a.in, &ans); err != nil {
		return authErr(errors.New("write confirmation cancelled"))
	}
	if strings.TrimSpace(ans) != "write" {
		return authErr(errors.New("write confirmation denied"))
	}
	return nil
}

func bindWriteFlag(cmd *cobra.Command, write *bool) {
	cmd.Flags().BoolVar(write, "write", false, "allow this mutating operation (interactive terminal; type write when prompted)")
}
