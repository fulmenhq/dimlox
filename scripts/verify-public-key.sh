#!/usr/bin/env bash

set -euo pipefail

FILE="${1:-}"

if [[ -z "${FILE}" ]]; then
    echo "usage: $0 path/to/public-key.asc" >&2
    exit 1
fi

if [[ ! -f "${FILE}" ]]; then
    echo "error: file not found: ${FILE}" >&2
    exit 1
fi

if rg -n "BEGIN PGP PRIVATE KEY BLOCK" "${FILE}" >/dev/null 2>&1; then
    echo "error: private key material detected in ${FILE}" >&2
    exit 1
fi

echo "[ok] Looks like a public-only key: ${FILE}"
