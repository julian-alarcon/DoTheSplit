#!/usr/bin/env bash
# Generates CycloneDX SBOMs for the api binary, the worker binary, and the app
# (Vue SPA) package. Outputs go to /sbom/ at repo root (gitignored). Published
# as release artifacts by .github/workflows/compliance.yml.
#
# Usage: ./scripts/generate-sbom.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/sbom"
mkdir -p "$OUT"

echo "→ CycloneDX SBOM: api"
(
  cd "$ROOT/api"
  go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0 app \
    -main ./cmd/api -json -output "$OUT/api.cdx.json" .
)

echo "→ CycloneDX SBOM: worker"
(
  cd "$ROOT/api"
  go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0 app \
    -main ./cmd/worker -json -output "$OUT/worker.cdx.json" .
)

echo "→ CycloneDX SBOM: app"
(
  cd "$ROOT/app"
  npx --yes @cyclonedx/cyclonedx-npm@4.2.1 \
    --output-file "$OUT/app.cdx.json" \
    --output-format JSON
)

echo "→ Verifying CycloneDX format"
for f in "$OUT/api.cdx.json" "$OUT/worker.cdx.json" "$OUT/app.cdx.json"; do
  if ! jq -e '.bomFormat == "CycloneDX"' "$f" >/dev/null; then
    echo "✗ $f is not a valid CycloneDX document" >&2
    exit 1
  fi
  echo "  $f ($(jq '.components | length' "$f") components)"
done

echo "Done. SBOMs in $OUT/"
