#!/usr/bin/env bash
# validate-prism.sh – Contract-Test: prüft ob alle prism-Klassen und CSS-Tokens
# die homeport nutzt im aktuellen prism.css Bundle vorhanden sind.
#
# Usage: ./scripts/validate-prism.sh
# Exit-Code 1 wenn Klassen/Tokens fehlen (blockiert Deployment).

set -euo pipefail

PRISM_CSS="assets/static/css/prism.css"
STYLE_CSS="assets/static/style.css"
TEMPLATES_DIR="assets/templates"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [ ! -f "${PRISM_CSS}" ]; then
  echo -e "${RED}FEHLER: ${PRISM_CSS} nicht gefunden – update-prism.sh zuerst ausführen${NC}"
  exit 1
fi

ERRORS=0
WARNINGS=0

VERSION=$(cat "assets/static/css/.prism-version" 2>/dev/null || echo "unbekannt")
echo "prism-ui Contract-Test (${VERSION})"
echo "Bundle: ${PRISM_CSS} ($(wc -l < "${PRISM_CSS}") Zeilen)"
echo "---"

# --- 1. CSS Custom Properties (Tokens) prüfen ---
echo "Prüfe CSS Custom Properties (var(--)…)…"

# Alle --tokens die in Templates und style.css via var() referenziert werden
USED_TOKENS=$(grep -roh 'var(--[^)]*)'  "${TEMPLATES_DIR}" "${STYLE_CSS}" 2>/dev/null \
  | grep -oE '\-\-[a-z][a-z0-9-]+' | sort -u)

MISSING_TOKENS=()
for token in ${USED_TOKENS}; do
  # Tokens die homeport selbst definiert (in style.css) überspringen
  if grep -qF -- "${token}:" "${STYLE_CSS}" 2>/dev/null; then
    continue
  fi
  # Token muss im prism Bundle definiert sein (fgrep: keine Option-Interpretation von --)
  if ! grep -qF -- "${token}" "${PRISM_CSS}"; then
    MISSING_TOKENS+=("${token}")
  fi
done

if [ ${#MISSING_TOKENS[@]} -eq 0 ]; then
  echo -e "  ${GREEN}✓ Alle CSS Tokens vorhanden${NC}"
else
  echo -e "  ${RED}✗ Fehlende CSS Tokens (${#MISSING_TOKENS[@]}):${NC}"
  for t in "${MISSING_TOKENS[@]}"; do
    echo "    FEHLT: ${t}"
    ERRORS=$((ERRORS + 1))
  done
fi

# --- 2. Prism-Klassen prüfen ---
echo "Prüfe prism CSS-Klassen…"

# Alle Klassen die im prism Bundle definiert sind
PRISM_CLASSES=$(grep -oE '\.[a-z][a-z0-9-]+' "${PRISM_CSS}" | tr -d '.' | sort -u)

# Alle Klassen die homeport selbst definiert (prism-unabhängig)
OWN_CLASSES=$(grep -oE '\.[a-z][a-z0-9-]+' "${STYLE_CSS}" 2>/dev/null | tr -d '.' | sort -u)

# Alle Klassen aus Templates (bereinigt: nur lowercase, min 3 Zeichen, kein Go-Keyword)
GO_KEYWORDS="if else end range with not and or eq ne lt gt le ge"
TEMPLATE_CLASSES=$(grep -roh 'class="[^"]*"' "${TEMPLATES_DIR}" \
  | grep -oE '"[^"]*"' | tr -d '"' | tr ' ' '\n' \
  | grep -E '^[a-z][a-z0-9-]{2,}$' \
  | grep -vxFf <(printf '%s\n' if else end range with not and or eq ne lt gt le ge) \
  | sort -u)

# Prüfen: Template-Klassen die aus prism kommen sollen aber fehlen
MISSING_CLASSES=()
for cls in ${TEMPLATE_CLASSES}; do
  if echo "${PRISM_CLASSES}" | grep -qx "${cls}"; then
    if ! grep -q "\.${cls}[^a-z0-9-]" "${PRISM_CSS}"; then
      MISSING_CLASSES+=("${cls}")
    fi
  fi
done

if [ ${#MISSING_CLASSES[@]} -eq 0 ]; then
  echo -e "  ${GREEN}✓ Alle prism CSS-Klassen vorhanden${NC}"
else
  echo -e "  ${RED}✗ Fehlende CSS-Klassen (${#MISSING_CLASSES[@]}):${NC}"
  for c in "${MISSING_CLASSES[@]}"; do
    echo "    FEHLT: .${c}"
    ERRORS=$((ERRORS + 1))
  done
fi

# --- 3. Unbekannte Klassen (weder prism noch eigene) melden ---
echo "Prüfe unbekannte Klassen…"

UNKNOWN=()
for cls in ${TEMPLATE_CLASSES}; do
  if ! echo "${PRISM_CLASSES}" | grep -qx "${cls}" && \
     ! echo "${OWN_CLASSES}" | grep -qx "${cls}"; then
    UNKNOWN+=("${cls}")
  fi
done

if [ ${#UNKNOWN[@]} -eq 0 ]; then
  echo -e "  ${GREEN}✓ Keine unbekannten Klassen${NC}"
else
  echo -e "  ${YELLOW}⚠ Unbekannte Klassen (${#UNKNOWN[@]}) – weder in prism.css noch style.css:${NC}"
  for c in "${UNKNOWN[@]}"; do
    echo "    WARN: .${c}"
    WARNINGS=$((WARNINGS + 1))
  done
fi

# --- Ergebnis ---
echo "---"
if [ ${ERRORS} -eq 0 ]; then
  echo -e "${GREEN}Contract-Test bestanden.${NC} Warnungen: ${WARNINGS}"
  exit 0
else
  echo -e "${RED}Contract-Test fehlgeschlagen: ${ERRORS} Fehler, ${WARNINGS} Warnungen${NC}"
  exit 1
fi
