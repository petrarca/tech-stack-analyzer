#!/usr/bin/env bash
#
# gen-licenses.sh — generate / verify third-party license compliance artifacts.
#
# Produces, for the stack-analyzer binary's actual dependency graph:
#   - THIRD_PARTY_NOTICES.md          (human-readable summary + component table)
#   - third_party/licenses/**         (full license text of every bundled module)
#
# Modes:
#   scripts/gen-licenses.sh            generate the artifacts in-place + run the
#                                      disallowed-license gate
#   scripts/gen-licenses.sh --check    CI gate: run the disallowed-license gate AND
#                                      verify the committed artifacts are not stale
#                                      (regenerate to a temp dir and diff). Writes
#                                      nothing. Non-zero exit on any failure.
#
# Output is deterministic (no timestamps) so the regen-and-diff staleness check is
# stable. Requires: go, go-licenses (installed locally if absent).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# The binary whose linked dependency graph defines what we must attribute.
TARGET="./cmd/scanner"
NOTICES_FILE="THIRD_PARTY_NOTICES.md"
LICENSES_DIR="third_party/licenses"
DISALLOWED="forbidden,restricted"   # strong copyleft (GPL/AGPL/etc.)
GOLICENSES_VERSION="v1.6.0"         # pinned for reproducibility

# --- ensure go-licenses is available ---
GOLICENSES_BIN="$(command -v go-licenses || true)"
if [ -z "$GOLICENSES_BIN" ]; then
    echo "go-licenses not found; installing ${GOLICENSES_VERSION} into ./bin..."
    mkdir -p "$ROOT_DIR/bin"
    GOBIN="$ROOT_DIR/bin" go install "github.com/google/go-licenses@${GOLICENSES_VERSION}"
    GOLICENSES_BIN="$ROOT_DIR/bin/go-licenses"
fi

# license_gate: fail on disallowed (strong-copyleft) licenses.
license_gate() {
    echo "Checking dependency licenses (disallowed: ${DISALLOWED})..."
    if ! "$GOLICENSES_BIN" check "$TARGET" --disallowed_types="$DISALLOWED" 2>/dev/null; then
        echo "ERROR: a dependency uses a disallowed (strong-copyleft) license." >&2
        echo "Review with: $GOLICENSES_BIN report $TARGET" >&2
        exit 1
    fi
    echo "License gate passed: no forbidden/restricted licenses."
}

# render_notices <report-csv> <notices-path>: deterministic markdown, no timestamp.
render_notices() {
    local csv="$1" out="$2" total
    total="$(wc -l < "$csv" | tr -d ' ')"
    {
        echo "# Third-Party Notices"
        echo
        echo "\`stack-analyzer\` includes third-party open-source software. The full"
        echo "license text for each component is reproduced under \`${LICENSES_DIR}/\`."
        echo
        echo "This file is generated — do not edit by hand. Regenerate with"
        echo "\`task licenses\` whenever dependencies change; \`task licenses:check\`"
        echo "verifies it is current."
        echo
        echo "## Summary by license"
        echo
        echo "| License | Count |"
        echo "|---------|-------|"
        cut -d',' -f3 "$csv" | sort | uniq -c | sort -rn \
            | while read -r count lic; do echo "| ${lic} | ${count} |"; done
        echo
        echo "Total: ${total} components. No strong-copyleft (GPL/LGPL/AGPL) and no"
        echo "unknown-license components are linked (enforced by the license gate)."
        echo
        echo "## Components"
        echo
        echo "| Component | License | License text |"
        echo "|-----------|---------|--------------|"
        while IFS=',' read -r module url license; do
            [ -z "$module" ] && continue
            echo "| ${module} | ${license} | ${url} |"
        done < "$csv"
    } > "$out"
}

# generate <notices-path> <licenses-dir>: full artifact generation into the targets.
generate() {
    local notices="$1" licdir="$2" csv
    csv="$(mktemp)"
    "$GOLICENSES_BIN" report "$TARGET" 2>/dev/null | sort > "$csv"
    render_notices "$csv" "$notices"
    rm -f "$csv"

    rm -rf "$licdir"
    mkdir -p "$(dirname "$licdir")"
    "$GOLICENSES_BIN" save "$TARGET" --save_path="$licdir" --force 2>/dev/null
    # go-licenses save copies whole module subtrees; keep only the legal files.
    find "$licdir" -type f \
        ! -iname 'license*' ! -iname 'licence*' ! -iname 'copying*' \
        ! -iname 'notice*'  ! -iname 'copyright*' -delete
    find "$licdir" -type d -empty -delete
}

# --- CI mode: gate + staleness diff, no writes ---
if [ "${1:-}" = "--check" ]; then
    license_gate
    echo "Verifying committed license artifacts are up to date..."
    TMP="$(mktemp -d)"
    trap 'rm -rf "$TMP"' EXIT
    generate "$TMP/$NOTICES_FILE" "$TMP/$LICENSES_DIR"
    stale=0
    if ! diff -q "$NOTICES_FILE" "$TMP/$NOTICES_FILE" >/dev/null 2>&1; then
        echo "STALE: $NOTICES_FILE differs from generated output." >&2; stale=1
    fi
    if ! diff -qr "$LICENSES_DIR" "$TMP/$LICENSES_DIR" >/dev/null 2>&1; then
        echo "STALE: $LICENSES_DIR differs from generated output." >&2; stale=1
    fi
    if [ "$stale" -ne 0 ]; then
        echo "ERROR: license artifacts are out of date. Run 'task licenses' and commit." >&2
        exit 1
    fi
    echo "License artifacts are up to date."
    exit 0
fi

# --- default: gate + generate in place ---
license_gate
echo "Generating ${NOTICES_FILE} and ${LICENSES_DIR}/ ..."
generate "$NOTICES_FILE" "$LICENSES_DIR"
echo "Done:"
echo "  - ${NOTICES_FILE} ($(wc -l < "$NOTICES_FILE" | tr -d ' ') lines)"
echo "  - ${LICENSES_DIR}/ ($(find "$LICENSES_DIR" -type f | wc -l | tr -d ' ') license files)"
