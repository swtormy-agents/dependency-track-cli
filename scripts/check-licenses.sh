#!/usr/bin/env bash
# Fail if any dependency uses GPL/AGPL/LGPL (forbidden/restricted), and print the full license report.
set -euo pipefail

cd "$(dirname "$0")/.."
# shellcheck source=scripts/go-licenses-env.sh
source scripts/go-licenses-env.sh

pkg="${DTRACK_LICENSE_PACKAGE}"

echo "=== go-licenses report (${GO_LICENSES_VERSION}, ${pkg}) ==="
dtrack_go_licenses report "${pkg}"

echo
echo "=== go-licenses check (disallowed_types=forbidden,restricted) ==="
dtrack_go_licenses check "${pkg}" --disallowed_types=forbidden,restricted
