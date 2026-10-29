# dtrack guide

## Setup

1. Point `DTRACK_URL` at the **API** base URL (scheme + host + port). In the default Docker Compose layout the API listens on **8081**; port **8080** is often the frontend only.
2. Create a **team API key** with the permissions your workflow needs (reuse Dependency-Track permission names: `VIEW_PORTFOLIO`, `VIEW_VULNERABILITY`, `VULNERABILITY_ANALYSIS`, `BOM_UPLOAD`, …).
3. Export `DTRACK_API_KEY` or set `DTRACK_API_KEY_FILE` to a `0600` file. Do not pass the key as a CLI argument.
4. Run `dtrack status` — exit `0` means the URL and key work for at least portfolio metrics.
5. **Writes:** only a human in an interactive terminal. Pass `--write` on the mutating command and type `write` when prompted. There is no persistent write mode in `config.json`.

## Command overview

| Command | HTTP | Permission (typical) |
|---------|------|----------------------|
| `dtrack version` | `GET /api/version` | none |
| `dtrack status` | `GET /api/v1/metrics/portfolio/current` | `VIEW_PORTFOLIO` |
| `dtrack project list` | `GET /api/v1/project` | `VIEW_PORTFOLIO` |
| `dtrack project get --uuid` | `GET /api/v1/project/{uuid}` | `VIEW_PORTFOLIO` |
| `dtrack project lookup --name [--version]` | `GET /api/v1/project/lookup` | `VIEW_PORTFOLIO` |
| `dtrack component list --project` | `GET /api/v1/component/project/{uuid}` | `VIEW_PORTFOLIO` |
| `dtrack finding list --project` | `GET /api/v1/finding/project/{uuid}` | `VIEW_VULNERABILITY` |
| `dtrack vuln get SOURCE/id` | `GET /api/v1/vulnerability/source/...` | `VIEW_VULNERABILITY` |
| `dtrack violation list --project` | `GET /api/v1/violation/project/{uuid}` | `VIEW_POLICY_VIOLATION` |
| `dtrack metrics portfolio` | `GET /api/v1/metrics/portfolio/current` | `VIEW_PORTFOLIO` |
| `dtrack metrics project --project` | `GET /api/v1/metrics/project/{uuid}/current` | `VIEW_PORTFOLIO` |
| `dtrack search --query` | `GET /api/v1/search` | `VIEW_PORTFOLIO` |
| `dtrack bom export --project` | `GET /api/v1/bom/cyclonedx/project/{uuid}` | `VIEW_PORTFOLIO` |
| `dtrack analysis set --write ...` | `PUT /api/v1/analysis` | `VULNERABILITY_ANALYSIS` + human `--write` |
| `dtrack bom upload --write ...` | `PUT /api/v1/bom` | `BOM_UPLOAD` + human `--write` |
| `dtrack raw METHOD /api/...` | arbitrary | varies; mutating calls need `--write` |

List commands honor `--page-size` (default 100) and `--max-items` (default 500).

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|----------------|-----|
| Exit `3`, `"type":"auth"`, HTTP 401 | Missing/invalid API key | Set `DTRACK_API_KEY`; create a new team key if it was rotated |
| Exit `3`, HTTP 403 | Key lacks permission | Add `VIEW_PORTFOLIO` / `VIEW_VULNERABILITY` / … on the team |
| Exit `3`, connection errors | Wrong URL or port | Use API port (`8081` in default Docker), check TLS and firewall |
| Exit `3`, read-only / no TTY | Agent or script tried a write | Human runs the command with `--write` in a real terminal |
| Exit `3`, API key in error text | Server echoed the key | Should be redacted in stderr; upgrade if you see the raw key |
| Exit `4`, raw blocked | Hit admin blocklist | Use typed commands; do not DELETE projects via CLI |
| Empty `project list` | Portfolio empty or filters | Verify projects in UI; increase caps only after narrowing |
| Huge JSON / slow lists | Large portfolio | Lower `--max-items`; filter findings by severity; use `search` |
| HTML in errors | Hitting UI port | Point `DTRACK_URL` at the API backend |
| Legacy `config.json` with `"access":"write"` | Old write mode | Ignored; use per-command `--write` instead |

## Agent skill

```bash
dtrack skill print
dtrack skill install --agent cursor
```

The embedded skill documents exit codes, untrusted content, and write gating.
