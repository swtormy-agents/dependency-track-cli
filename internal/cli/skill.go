package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed SKILL.md
var skillMD string

var skillDirs = map[string]string{
	"claude": ".claude/skills",
	"cursor": ".cursor/skills",
	"grok":   ".grok/skills",
	"agents": ".agents/skills",
}

func (a *app) skillCmd() *cobra.Command {
	skill := &cobra.Command{
		Use:   "skill",
		Short: "Print or install the agent skill bundled in this binary",
	}
	skill.AddCommand(&cobra.Command{
		Use:   "print",
		Short: "Print SKILL.md to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(a.out, skillMD)
			return err
		},
	})
	var agent, dir string
	var global bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Write SKILL.md for an agent (project-local by default)",
		Example: `  dtrack skill install
  dtrack skill install --global
  dtrack skill install --agent cursor --global`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := dir
			if target == "" {
				sub, ok := skillDirs[agent]
				if !ok {
					return usageErr("--agent must be one of claude, cursor, grok, agents")
				}
				base, err := os.Getwd()
				if global {
					base, err = os.UserHomeDir()
				}
				if err != nil {
					return runtimeErr(err)
				}
				target = filepath.Join(base, sub, "dtrack")
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return runtimeErr(err)
			}
			file := filepath.Join(target, "SKILL.md")
			if err := os.WriteFile(file, []byte(skillMD), 0o644); err != nil {
				return runtimeErr(err)
			}
			fmt.Fprintln(a.out, file)
			return nil
		},
	}
	install.Flags().StringVar(&agent, "agent", "claude", "claude, cursor, grok or agents")
	install.Flags().BoolVar(&global, "global", false, "install into the home directory")
	install.Flags().StringVar(&dir, "dir", "", "exact target directory")
	skill.AddCommand(install)
	return skill
}
