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
3. **Run write commands yourself** in an interactive terminal (agents are read-only). Each mutating command needs `--write` and typing `write` when prompted:

   ```bash
   dtrack analysis set --write --project ... --component ... --vuln ... --state NOT_AFFECTED
   dtrack bom upload --write --project ... --file bom.json
   ```

4. **Install the agent skill** (optional):

   ```bash
   dtrack skill install --agent cursor
   ```

## Safety

- JSON on stdout; one JSON error line on stderr. Exit codes: `0` ok, `1` runtime/API, `2` usage or config (invalid local setup, including API keys shorter than 16 characters), `3` auth (missing key, HTTP 401/403, or write refused/cancelled), `4` blocked by `raw` blocklist.
- Agents stay read-only. `config.json` does not grant write access. Mutating commands require `--write`, a TTY, and typed confirmation in the same invocation.
- Third-party text in responses can be wrapped with `"untrusted": true` (default `--mark-untrusted`).
- `raw` blocks destructive admin routes (DELETE on projects/teams/users, config changes).

See [docs/GUIDE.md](docs/GUIDE.md) for setup, commands, and troubleshooting.

## License

MIT — see [LICENSE](LICENSE). Third-party notices in [NOTICE](NOTICE).
