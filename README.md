# dependency-track-cli

`dtrack` is a small Go CLI for the [Dependency-Track](https://dependencytrack.org/) REST API. It is aimed at humans and AI agents that need structured JSON instead of an MCP server (see [DependencyTrack#5791](https://github.com/DependencyTrack/dependency-track/issues/5791)).

## Install

```bash
go install github.com/swtormy-agents/dependency-track-cli/cmd/dtrack@latest
```

Or download a release archive from GitHub (Linux/macOS/Windows, amd64/arm64).

## Quick start

```bash
export DTRACK_URL=https://dtrack.example:8081    # API port (often 8081 in Docker), not the UI port
export DTRACK_API_KEY=...                        # team API key — never pass on the command line

dtrack status
dtrack project list --max-items 20
dtrack finding list --project <uuid> --severity HIGH
```

Pipe the API key instead of exporting it:

```bash
echo "$DTRACK_API_KEY" | dtrack status
```

## Human steps (once per machine)

1. **Create a team API key** in Dependency-Track with the permissions you need:
   - Read: `VIEW_PORTFOLIO`, `VIEW_VULNERABILITY`, `VIEW_POLICY_VIOLATION` (typical triage)
   - Write: add `VULNERABILITY_ANALYSIS` for analysis updates, `BOM_UPLOAD` for BOM upload
2. **Store the key** in your environment or a `chmod 600` file referenced by `DTRACK_API_KEY_FILE`.
3. **Enable writes** when you intend to change analysis or upload BOMs (agents stay read-only until you do this):

   ```bash
   dtrack access write    # interactive terminal — type `write` to confirm
   ```

4. **Install the agent skill** (optional):

   ```bash
   dtrack skill install --agent cursor
   ```

## Safety

- JSON on stdout; one JSON error line on stderr. Exit codes: `0` ok, `1` runtime/API, `2` usage, `3` auth, `4` blocked by local access policy.
- Read-only by default. `bom upload`, `analysis set`, and mutating `raw` calls require `dtrack access write` from a human at a TTY.
- Third-party text in responses can be wrapped with `"untrusted": true` (default `--mark-untrusted`).
- `raw` blocks destructive admin routes (DELETE on projects/teams/users, config changes).

See [docs/GUIDE.md](docs/GUIDE.md) for setup, commands, and troubleshooting.

## License

MIT — see [LICENSE](LICENSE). Third-party notices in [NOTICE](NOTICE).
