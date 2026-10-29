#!/usr/bin/env bash
# Cross-compiles dtrack for Linux and Windows (amd64, arm64) into ./dist and writes checksums.txt.
# darwin targets omitted until macOS CI is green (see .github/workflows/ci.yaml).
set -euo pipefail

cd "$(dirname "$0")/.."
version="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
version="${version#v}"
rm -rf dist
mkdir -p dist

# shellcheck source=scripts/go-licenses-env.sh
source scripts/go-licenses-env.sh

license_stage="dist/.license-staging"
rm -rf "${license_stage}"
mkdir -p "${license_stage}"

dtrack_go_licenses report "${DTRACK_LICENSE_PACKAGE}" >"${license_stage}/report.csv" 2>/dev/null
dtrack_go_licenses save "${DTRACK_LICENSE_PACKAGE}" --save_path="${license_stage}/licenses" 2>/dev/null

{
  echo "Third-party licenses for dtrack ${version}"
  echo "Generated with github.com/google/go-licenses ${GO_LICENSES_VERSION}"
  echo
  echo "=== Dependency report (CSV) ==="
  cat "${license_stage}/report.csv"
  echo
  echo "=== License texts ==="
  while IFS= read -r -d '' f; do
    echo
    echo "--- ${f#"${license_stage}/licenses/"} ---"
    cat "$f"
  done < <(find "${license_stage}/licenses" -type f \( -name 'LICENSE*' -o -name 'COPYING*' \) -print0 | sort -z)
} >"${license_stage}/THIRD_PARTY_NOTICES"

mkdir -p "${license_stage}/apache-notices"
while IFS=, read -r mod _ lic; do
  [ -n "$mod" ] || continue
  [ "$lic" = "Apache-2.0" ] || continue
  mod_dir="$(go list -m -f '{{.Dir}}' "${mod}" 2>/dev/null || true)"
  [ -n "${mod_dir}" ] || continue
  while IFS= read -r -d '' notice; do
    rel="${mod}$(basename "$notice")"
    safe="$(echo "${rel}" | tr '/@.' '___')"
    cp "$notice" "${license_stage}/apache-notices/${safe}"
  done < <(find "${mod_dir}" -maxdepth 1 -type f \( -iname 'NOTICE' -o -iname 'NOTICE.txt' \) -print0 2>/dev/null)
done < <(tail -n +1 "${license_stage}/report.csv")

# linux/amd64 linux/arm64 windows/amd64 windows/arm64
# darwin/amd64 darwin/arm64 — only after a green macOS CI run
targets="linux/amd64 linux/arm64 windows/amd64 windows/arm64"
for t in $targets; do
  os="${t%/*}"
  arch="${t#*/}"
  name="dtrack_${os}_${arch}"
  stage="dist/${name}"
  mkdir -p "$stage"
  bin="dtrack"
  [ "$os" = "windows" ] && bin="dtrack.exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X github.com/swtormy-agents/dependency-track-cli/internal/cli.Version=${version}" \
    -o "${stage}/${bin}" ./cmd/dtrack
  cp LICENSE NOTICE "${license_stage}/THIRD_PARTY_NOTICES" "$stage/"
  if [ -d "${license_stage}/licenses" ]; then
    cp -r "${license_stage}/licenses" "${stage}/third-party-licenses"
  fi
  if [ -d "${license_stage}/apache-notices" ] && [ -n "$(ls -A "${license_stage}/apache-notices" 2>/dev/null || true)" ]; then
    cp -r "${license_stage}/apache-notices" "${stage}/apache-notices"
  fi
  cp README.md "$stage/"
  cp -r docs "$stage/docs"
  if [ "$os" = "windows" ]; then
    if command -v zip >/dev/null; then
      (cd dist && zip -qr "${name}.zip" "$name")
    else
      (cd dist && python3 -m zipfile -c "${name}.zip" "$name")
    fi
  else
    tar -C dist -czf "dist/${name}.tar.gz" "$name"
  fi
  rm -rf "$stage"
done

rm -rf "${license_stage}"

cd dist
if command -v sha256sum >/dev/null; then
  sha256sum -- *.tar.gz *.zip > checksums.txt 2>/dev/null || true
else
  shasum -a 256 -- *.tar.gz *.zip > checksums.txt 2>/dev/null || true
fi
ls -la
