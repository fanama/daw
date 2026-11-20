#!/usr/bin/env bash
# Enforces the repository readability caps: 300 lines per non-test file and
# 40 lines per top-level function. Exits non-zero on any violation.
set -euo pipefail

cd "$(dirname "$0")/.."

failures=0
files=$(find . -name '*.go' -not -path './.git/*' -not -path './vendor/*' -not -name '*_test.go')

for file in $files; do
  lines=$(wc -l < "$file" | tr -d ' ')
  if [ "$lines" -gt 300 ]; then
    echo "file cap exceeded: $file ($lines lines)"
    failures=$((failures + 1))
  fi
done

for file in $files; do
  report=$(awk '
    /^func / { name=$0; start=NR; depth=0; opened=0 }
    name != "" {
      opens = gsub(/{/, "{")
      closes = gsub(/}/, "}")
      depth += opens - closes
      if (opens > 0) opened = 1
      if (opened && depth <= 0) {
        spanned = NR - start + 1
        if (spanned > 40) {
          printf "function cap exceeded: %s:%d %s (%d lines)\n", FILENAME, start, name, spanned
        }
        name = ""
      }
    }
  ' "$file")
  if [ -n "$report" ]; then
    echo "$report"
    failures=$((failures + 1))
  fi
done

if [ "$failures" -gt 0 ]; then
  echo "caps: $failures violation(s)"
  exit 1
fi
echo "caps ok: every file <= 300 lines, every function <= 40 lines"
