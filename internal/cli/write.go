package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const writePrompt = "This command changes Dependency-Track. Type 'write' to confirm: "

// requireHumanWrite gates mutating commands: --write flag, interactive TTY, typed confirmation.
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
	fmt.Fprint(a.errOut, writePrompt)
	var ans string
	if _, err := fmt.Fscanln(a.in, &ans); err != nil {
		return usageErr("cancelled")
	}
	if strings.TrimSpace(ans) != "write" {
		return usageErr("cancelled")
	}
	return nil
}

func bindWriteFlag(cmd *cobra.Command, write *bool) {
	cmd.Flags().BoolVar(write, "write", false, "allow this mutating operation (interactive terminal; type write when prompted)")
}
