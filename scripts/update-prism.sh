#!/usr/bin/env bash
# update-prism.sh – PRISM UI in homeport aktualisieren
# Usage: ./scripts/update-prism.sh [TAG]
# Beispiel: ./scripts/update-prism.sh v2.0.0
# Ohne Argument: fragt Gitea nach dem neuesten Release-Tag

set -euo pipefail

GITEA_HOST="git.zk35.de"
PRISM_REPO="secalpha/prism-ui"
CSS_DIR="assets/static/css"
JS_DIR="assets/static/js"

# Tag ermitteln
if [ -n "${1:-}" ]; then
  TAG="$1"
else
  echo "Aktuellsten Release abfragen…"
  TAG=$(curl -sf "https://${GITEA_HOST}/api/v1/repos/${PRISM_REPO}/releases?limit=1&token=${GITEA_TOKEN:-}" \
    | python3 -c "import sys,json; releases=json.load(sys.stdin); print(releases[0]['tag_name']) if releases else exit(1)" \
    2>/dev/null) || { echo "Fehler: Kein Release gefunden. Tag manuell angeben."; exit 1; }
fi

BASE_URL="https://${GITEA_HOST}/${PRISM_REPO}/releases/download/${TAG}"

echo "PRISM UI ${TAG} → homeport"
echo "Von: ${BASE_URL}"

mkdir -p "${CSS_DIR}" "${JS_DIR}"

echo "  → ${CSS_DIR}/prism.css"
curl -fsSL "${BASE_URL}/prism-bundle.css" -o "${CSS_DIR}/prism.css"

echo "  → ${JS_DIR}/prism.js"
curl -fsSL "${BASE_URL}/prism.js" -o "${JS_DIR}/prism.js"

# Versionsdatei schreiben damit man später weiß was drin ist
echo "${TAG}" > "${CSS_DIR}/.prism-version"

echo ""
echo "Fertig. Aktive Version: ${TAG}"
echo "Commit: git add ${CSS_DIR}/prism.css ${JS_DIR}/prism.js ${CSS_DIR}/.prism-version"
