package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/dependency-track-cli/internal/dt"
)

func (a *app) bomCmd() *cobra.Command {
	bom := &cobra.Command{Use: "bom", Short: "Software bill of materials"}
	exportProject := ""
	export := &cobra.Command{
		Use:   "export",
		Short: "Export CycloneDX BOM for a project (read)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if exportProject == "" {
				return usageErr("--project is required")
			}
			path := "/api/v1/bom/cyclonedx/project/" + exportProject
			return a.simpleGet(path, map[string]string{"download": "true"})(cmd, args)
		},
	}
	export.Flags().StringVar(&exportProject, "project", "", "project UUID")

	uploadProject := ""
	uploadFile := ""
	var uploadWrite bool
	upload := &cobra.Command{
		Use:   "upload",
		Short: "Upload a BOM (human: --write + interactive confirmation)",
		Long:  "PUT /api/v1/bom with multipart form. Requires --write, a TTY, and team permission BOM_UPLOAD.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireHumanWrite(cmd, "bom upload"); err != nil {
				return err
			}
			if uploadProject == "" || uploadFile == "" {
				return usageErr("--project and --file are required")
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if a.dryRun {
				return a.emitValue(map[string]any{
					"method":  http.MethodPut,
					"url":     c.BaseURL + "/api/v1/bom",
					"project": uploadProject,
					"file":    uploadFile,
				})
			}
			raw, _, err := c.UploadBOM(ctx, uploadProject, uploadFile)
			if err != nil {
				return a.wrapAPI(err)
			}
			return a.emitUntrusted(raw)
		},
	}
	upload.Flags().StringVar(&uploadProject, "project", "", "project UUID")
	upload.Flags().StringVar(&uploadFile, "file", "", "path to CycloneDX JSON/XML file")
	bindWriteFlag(upload, &uploadWrite)

	bom.AddCommand(export, upload)
	return bom
}

func (a *app) analysisRootCmd() *cobra.Command {
	analysis := &cobra.Command{Use: "analysis", Short: "Vulnerability analysis (write gated)"}
	analysis.AddCommand(a.analysisSetCmd())
	return analysis
}

func (a *app) analysisSetCmd() *cobra.Command {
	var project, component, vuln, state, justification, comment string
	var writeFlag bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set vulnerability analysis (human: --write + interactive confirmation)",
		Long:  "Updates analysis via PUT /api/v1/analysis.",
		Example: `  dtrack analysis set --project UUID --component UUID --vuln UUID \
    --state NOT_AFFECTED --justification CODE_NOT_PRESENT --comment "not in our build"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.requireHumanWrite(cmd, "analysis set"); err != nil {
				return err
			}
			if project == "" || component == "" || vuln == "" || state == "" {
				return usageErr("--project, --component, --vuln, and --state are required")
			}
			body := map[string]any{
				"project":       project,
				"component":     component,
				"vulnerability": vuln,
				"analysisState": state,
			}
			if justification != "" {
				body["analysisJustification"] = justification
			}
			if comment != "" {
				body["comment"] = comment
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if a.dryRun {
				return a.emitValue(dt.DryRun(http.MethodPut, c.BaseURL, "/api/v1/analysis", nil, body))
			}
			raw, _, err := c.JSON(ctx, http.MethodPut, "/api/v1/analysis", body)
			if err != nil {
				return a.wrapAPI(err)
			}
			return a.emitUntrusted(raw)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project UUID")
	cmd.Flags().StringVar(&component, "component", "", "component UUID")
	cmd.Flags().StringVar(&vuln, "vuln", "", "vulnerability UUID")
	cmd.Flags().StringVar(&state, "state", "", "analysis state (NOT_AFFECTED, FALSE_POSITIVE, EXPLOITABLE, ...)")
	cmd.Flags().StringVar(&justification, "justification", "", "analysis justification when required")
	cmd.Flags().StringVar(&comment, "comment", "", "comment stored with the analysis")
	bindWriteFlag(cmd, &writeFlag)
	return cmd
}

func (a *app) rawCmd() *cobra.Command {
	var bodyJSON string
	var writeFlag bool
	cmd := &cobra.Command{
		Use:   "raw <METHOD> <path>",
		Short: "Call an API path (escape hatch with blocklist)",
		Long:  "Paths must start with /api. Some admin routes are always blocked; mutating methods need --write and interactive confirmation.",
		Example: `  dtrack raw GET /api/v1/project?pageNumber=1&pageSize=1`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			if err := checkRawAllowed(method, path); err != nil {
				return err
			}
			if rawNeedsWrite(method) {
				if err := a.requireHumanWrite(cmd, "raw "+method+" "+path); err != nil {
					return err
				}
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if path == "" || path[0] != '/' {
				path = "/" + path
			}
			if a.dryRun {
				var body any
				if bodyJSON != "" {
					_ = json.Unmarshal([]byte(bodyJSON), &body)
				}
				return a.emitValue(dt.DryRun(method, c.BaseURL, path, nil, body))
			}
			var raw json.RawMessage
			if bodyJSON != "" {
				var body any
				if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
					return usageErr("--body: %v", err)
				}
				raw, _, err = c.JSON(ctx, method, path, body)
			} else {
				raw, _, err = c.Do(ctx, method, path, nil, nil)
			}
			if err != nil {
				return a.wrapAPI(err)
			}
			return a.emitUntrusted(raw)
		},
	}
	cmd.Flags().StringVar(&bodyJSON, "body", "", "JSON body for POST/PUT/PATCH")
	bindWriteFlag(cmd, &writeFlag)
	return cmd
}

func checkRawAllowed(method, path string) error {
	p := strings.ToLower(path)
	if method == http.MethodDelete {
		for _, frag := range []string{"/project/", "/team/", "/user/", "/apikey", "/acl", "/permission"} {
			if strings.Contains(p, frag) {
				return blockedErr(fmt.Errorf("raw %s %s is permanently blocked", method, path))
			}
		}
	}
	if method != http.MethodGet && method != http.MethodHead {
		for _, frag := range []string{"/ldap", "/oidc", "/configproperty", "/integration"} {
			if strings.Contains(p, frag) {
				return blockedErr(fmt.Errorf("raw %s %s is blocked (system configuration)", method, path))
			}
		}
	}
	return nil
}

func rawNeedsWrite(method string) bool {
	return method != http.MethodGet && method != http.MethodHead
}
