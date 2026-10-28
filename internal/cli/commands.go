package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/dependency-track-cli/internal/config"
	"github.com/swtormy-agents/dependency-track-cli/internal/dt"
	"golang.org/x/term"
)

var isTerminal = func(r ioReader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

type ioReader = interface {
	Read(p []byte) (n int, err error)
}

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Server version (GET /api/version)",
		Args:  cobra.NoArgs,
		RunE:  a.simpleGet("/api/version", nil),
	}
}

func (a *app) statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Verify DTRACK_URL and API key",
		Long:  `Uses GET /api/v1/metrics/portfolio/current as an authenticated health check.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := a.loadAccess(); err != nil {
				return err
			}
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			path := "/api/v1/metrics/portfolio/current"
			if a.dryRun {
				return a.emitValue(map[string]any{
					"request": dt.DryRun(http.MethodGet, c.BaseURL, path, nil, nil),
					"access":  a.access.Access,
				})
			}
			raw, _, err := c.Get(ctx, path, nil)
			if err != nil {
				return a.wrapAPI(err)
			}
			var metrics json.RawMessage = raw
			return a.emitValue(map[string]any{"ok": true, "url": c.BaseURL, "access": a.access.Access, "metrics": metrics})
		},
	}
}

func (a *app) simpleGet(path string, query map[string]string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := a.client(ctx)
		if err != nil {
			return err
		}
		if a.dryRun {
			return a.emitValue(dt.DryRun(http.MethodGet, c.BaseURL, path, urlValues(query), nil))
		}
		raw, _, err := c.Get(ctx, path, urlValues(query))
		if err != nil {
			return a.wrapAPI(err)
		}
		return a.emitUntrusted(raw)
	}
}

func (a *app) accessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access [read|write]",
		Short: "Show or change local write access (write needs a human at a TTY)",
		Long: `Without an argument, shows the local access level stored in your config directory.

  read   (default) only read commands work; BOM upload and analysis changes exit 4
  write  allows analysis set and BOM upload

Switching to write requires an interactive terminal and typing "write".`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.loadAccess(); err != nil {
				return err
			}
			if len(args) == 1 {
				switch args[0] {
				case config.AccessRead:
				case config.AccessWrite:
					if !isTerminal(a.in) {
						return blockedErr(fmt.Errorf("write access can only be granted in an interactive terminal; ask the user to run `dtrack access write` themselves"))
					}
					fmt.Fprintf(a.errOut, "Allow dtrack to upload BOMs and set vulnerability analysis on %s? Type 'write' to confirm: ", a.getenv("DTRACK_URL"))
					var ans string
					fmt.Fscanln(a.in, &ans)
					ans = strings.TrimSpace(ans)
					if ans != "write" {
						return usageErr("cancelled")
					}
				default:
					return usageErr("access must be read or write")
				}
				a.access.Access = args[0]
				if err := config.Save(a.getenv, a.access); err != nil {
					return runtimeErr(err)
				}
			}
			return a.emitValue(map[string]any{"access": a.access.Access})
		},
	}
	return cmd
}

func (a *app) projectCmd() *cobra.Command {
	project := &cobra.Command{Use: "project", Short: "Projects (VIEW_PORTFOLIO)"}
	var name, version, uuid string
	list := &cobra.Command{
		Use:   "list",
		Short: "List projects (paginated, capped by --max-items)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.listGET(cmd.Context(), "/api/v1/project", nil)
		},
	}
	get := &cobra.Command{
		Use:   "get",
		Short: "Get one project by UUID",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if uuid == "" {
				return usageErr("--uuid is required")
			}
			return a.simpleGet("/api/v1/project/"+uuid, nil)(cmd, args)
		},
	}
	get.Flags().StringVar(&uuid, "uuid", "", "project UUID")
	lookup := &cobra.Command{
		Use:   "lookup",
		Short: "Lookup project by name and version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return usageErr("--name is required")
			}
			q := map[string]string{"name": name}
			if version != "" {
				q["version"] = version
			}
			return a.simpleGet("/api/v1/project/lookup", q)(cmd, args)
		},
	}
	lookup.Flags().StringVar(&name, "name", "", "project name")
	lookup.Flags().StringVar(&version, "version", "", "project version")
	project.AddCommand(list, get, lookup)
	return project
}

func (a *app) componentCmd() *cobra.Command {
	component := &cobra.Command{Use: "component", Short: "Components in a project"}
	var project string
	list := &cobra.Command{
		Use:   "list",
		Short: "List components in a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return usageErr("--project is required")
			}
			return a.listGET(cmd.Context(), "/api/v1/component/project/"+project, nil)
		},
	}
	list.Flags().StringVar(&project, "project", "", "project UUID")
	component.AddCommand(list)
	return component
}

func (a *app) findingCmd() *cobra.Command {
	finding := &cobra.Command{Use: "finding", Short: "Vulnerability findings"}
	var project, severity string
	var suppressed bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List findings for a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return usageErr("--project is required")
			}
			q := map[string]string{}
			if severity != "" {
				q["severity"] = severity
			}
			if cmd.Flags().Changed("suppressed") {
				q["suppressed"] = fmt.Sprintf("%t", suppressed)
			}
			return a.listGET(cmd.Context(), "/api/v1/finding/project/"+project, q)
		},
	}
	list.Flags().StringVar(&project, "project", "", "project UUID")
	list.Flags().StringVar(&severity, "severity", "", "filter by severity (CRITICAL, HIGH, ...)")
	list.Flags().BoolVar(&suppressed, "suppressed", false, "include suppressed findings")
	finding.AddCommand(list)
	return finding
}

func (a *app) vulnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vuln get <source>/<vulnId>",
		Short: "Get a vulnerability (GET /api/v1/vulnerability/source/{source}/vuln/{id})",
		Example: `  dtrack vuln get NVD/CVE-2024-1234
  dtrack vuln get github/GO-2024-0001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, id, ok := strings.Cut(args[0], "/")
			if !ok || src == "" || id == "" {
				return usageErr("vuln id must be source/vulnId (example: NVD/CVE-2024-1234)")
			}
			path := fmt.Sprintf("/api/v1/vulnerability/source/%s/vuln/%s", urlPathEscape(src), urlPathEscape(id))
			return a.simpleGet(path, nil)(cmd, args)
		},
	}
}

func urlPathEscape(s string) string {
	return strings.ReplaceAll(s, " ", "%20")
}

func (a *app) violationCmd() *cobra.Command {
	violation := &cobra.Command{Use: "violation", Short: "Policy violations"}
	var project string
	list := &cobra.Command{
		Use:   "list",
		Short: "List policy violations for a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return usageErr("--project is required")
			}
			return a.listGET(cmd.Context(), "/api/v1/violation/project/"+project, nil)
		},
	}
	list.Flags().StringVar(&project, "project", "", "project UUID")
	violation.AddCommand(list)
	return violation
}

func (a *app) metricsCmd() *cobra.Command {
	metrics := &cobra.Command{Use: "metrics", Short: "Portfolio and project metrics"}
	var project string
	metrics.AddCommand(&cobra.Command{
		Use:   "portfolio",
		Short: "Current portfolio metrics",
		Args:  cobra.NoArgs,
		RunE:  a.simpleGet("/api/v1/metrics/portfolio/current", nil),
	})
	metrics.AddCommand(&cobra.Command{
		Use:   "project",
		Short: "Current metrics for one project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return usageErr("--project is required")
			}
			return a.simpleGet("/api/v1/metrics/project/"+project+"/current", nil)(cmd, args)
		},
	})
	metrics.Commands()[1].Flags().StringVar(&project, "project", "", "project UUID")
	return metrics
}

func (a *app) searchCmd() *cobra.Command {
	var query string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Global search (GET /api/v1/search)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return usageErr("--query is required")
			}
			return a.simpleGet("/api/v1/search", map[string]string{"query": query})(cmd, args)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search string")
	return cmd
}

func (a *app) listGET(ctx context.Context, path string, query map[string]string) error {
	c, err := a.client(ctx)
	if err != nil {
		return err
	}
	q := urlValues(query)
	if a.dryRun {
		q.Set("pageSize", fmt.Sprintf("%d", a.pageSize))
		q.Set("pageNumber", "1")
		return a.emitValue(map[string]any{
			"request":   dt.DryRun(http.MethodGet, c.BaseURL, path, q, nil),
			"max_items": a.maxItems,
		})
	}
	raw, err := c.PaginateGET(ctx, path, q, a.pageSize, a.maxItems)
	if err != nil {
		return a.wrapAPI(err)
	}
	return a.emitUntrusted(raw)
}

func jsonUnmarshal(raw json.RawMessage, v any) error {
	return json.Unmarshal(raw, v)
}
