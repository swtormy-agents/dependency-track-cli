package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/dependency-track-cli/internal/dt"
)

const (
	exitRuntime = 1
	exitUsage   = 2
	exitAuth    = 3
	exitBlocked = 4
)

type exitError struct {
	code int
	kind string
	err  error
	api  *dt.APIError
}

func (e *exitError) Error() string { return e.err.Error() }

func usageErr(format string, a ...any) error {
	return &exitError{code: exitUsage, kind: "usage", err: fmt.Errorf(format, a...)}
}

func runtimeErr(err error) error {
	return &exitError{code: exitRuntime, kind: "runtime", err: err}
}

func authErr(err error) error {
	return &exitError{code: exitAuth, kind: "auth", err: err}
}

func blockedErr(err error) error {
	return &exitError{code: exitBlocked, kind: "blocked", err: err}
}

var Version = "dev"

func versionString() string {
	v := Version
	if v != "dev" {
		return v
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		rev, dirty := "", ""
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					dirty = "+dirty"
				}
			}
		}
		if len(rev) > 7 {
			rev = rev[:7]
		}
		if rev != "" {
			return "dev-" + rev + dirty
		}
	}
	return v
}

func staleHint(err error) string {
	m := err.Error()
	if strings.Contains(m, "unknown flag") || strings.Contains(m, "unknown command") || strings.Contains(m, "unknown shorthand") {
		return fmt.Sprintf(" (this is dtrack %s; rebuild or `go install github.com/swtormy-agents/dependency-track-cli/cmd/dtrack@main`)", versionString())
	}
	return ""
}

type app struct {
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
	getenv  func(string) string
	dryRun  bool
	jq      string
	pretty  bool
	baseURL string
	timeout time.Duration
	pageSize int
	maxItems int
	untrusted bool
	apiKey  string
}

func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	a := &app{in: stdin, out: stdout, errOut: stderr, getenv: getenv, pageSize: 100, maxItems: 500}
	root := a.rootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		if _, ok := err.(*exitError); !ok {
			err = usageErr("%s%s", err, staleHint(err))
		}
		return a.reportError(err)
	}
	return 0
}

func (a *app) reportError(err error) int {
	ee, ok := err.(*exitError)
	if !ok {
		ee = &exitError{code: exitUsage, kind: "usage", err: err}
	}
	detail := map[string]any{"type": ee.kind, "message": a.redact(ee.err.Error())}
	if ee.api != nil {
		detail["status"] = ee.api.Status
		if ee.api.Message != "" {
			detail["message"] = a.redact(ee.api.Message)
		}
	}
	b, _ := marshal(map[string]any{"ok": false, "error": detail})
	fmt.Fprintln(a.errOut, string(b))
	return ee.code
}

func (a *app) redact(s string) string {
	return (&dt.Client{APIKey: a.apiKey}).Redact(s)
}

func (a *app) resolveAPIKey() error {
	if a.apiKey != "" {
		return nil
	}
	a.apiKey = strings.TrimSpace(a.getenv("DTRACK_API_KEY"))
	if a.apiKey != "" {
		return nil
	}
	if a.getenv("DTRACK_API_KEY_FILE") != "" {
		b, err := os.ReadFile(a.getenv("DTRACK_API_KEY_FILE"))
		if err != nil {
			return authErr(fmt.Errorf("DTRACK_API_KEY_FILE: %w", err))
		}
		a.apiKey = strings.TrimSpace(string(b))
		if a.apiKey != "" {
			return nil
		}
	}
	rd := bufio.NewReader(a.in)
	line, err := rd.ReadString('\n')
	if err != nil && line == "" {
		return authErr(errors.New("DTRACK_API_KEY is not set; export it or pipe the team API key on stdin"))
	}
	a.apiKey = strings.TrimSpace(line)
	if a.apiKey == "" {
		return authErr(errors.New("empty API key on stdin"))
	}
	return nil
}

func (a *app) client(ctx context.Context) (*dt.Client, error) {
	if err := a.resolveAPIKey(); err != nil {
		return nil, err
	}
	base := a.baseURL
	if base == "" {
		base = strings.TrimSpace(a.getenv("DTRACK_URL"))
	}
	if base == "" {
		return nil, authErr(errors.New("DTRACK_URL is not set (example: https://dtrack.example:8081)"))
	}
	return &dt.Client{
		BaseURL: base,
		APIKey:  a.apiKey,
		HTTP:    &http.Client{Timeout: a.timeout},
	}, nil
}

func (a *app) wrapAPI(err error) error {
	var api *dt.APIError
	if errors.As(err, &api) {
		code, kind := exitRuntime, "api"
		if api.IsAuth() {
			code, kind = exitAuth, "auth"
		}
		return &exitError{code: code, kind: kind, err: api, api: api}
	}
	return runtimeErr(err)
}

func (a *app) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "dtrack",
		Short:         "Dependency-Track REST API from the shell, built for AI agents",
		Long:          rootLong,
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageErr("%s%s", err, staleHint(err))
	})
	pf := root.PersistentFlags()
	pf.BoolVar(&a.dryRun, "dry-run", false, "print the HTTP request instead of sending it")
	pf.StringVar(&a.jq, "jq", "", "filter JSON output with a jq expression")
	pf.BoolVar(&a.pretty, "pretty", false, "indent JSON output")
	pf.StringVar(&a.baseURL, "base-url", "", "Dependency-Track base URL (default $DTRACK_URL)")
	pf.DurationVar(&a.timeout, "http-timeout", 60*time.Second, "HTTP timeout per request")
	pf.IntVar(&a.pageSize, "page-size", 100, "page size for list commands (server default capped)")
	pf.IntVar(&a.maxItems, "max-items", 500, "maximum items returned by list commands")
	pf.BoolVar(&a.untrusted, "mark-untrusted", true, "tag third-party text fields with untrusted:true in JSON output")

	root.AddCommand(
		a.versionCmd(),
		a.statusCmd(),
		a.accessCmd(),
		a.projectCmd(),
		a.componentCmd(),
		a.findingCmd(),
		a.vulnCmd(),
		a.violationCmd(),
		a.metricsCmd(),
		a.searchCmd(),
		a.bomCmd(),
		a.analysisRootCmd(),
		a.rawCmd(),
		a.skillCmd(),
	)
	return root
}

const rootLong = `dtrack calls the Dependency-Track REST API from the shell. JSON on stdout; errors on stderr.

Setup (human, once)
  export DTRACK_URL=https://dtrack.example:8081
  export DTRACK_API_KEY=...          team API key (never pass on the command line)

Read-only for agents. Mutating commands need --write, an interactive terminal, and typing "write" when prompted.

Exit codes: 0 ok, 1 runtime/API, 2 usage, 3 auth (missing key, HTTP 401/403, or write refused), 4 blocked by raw blocklist.`
