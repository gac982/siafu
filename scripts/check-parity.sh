#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Siafu contributors
#
# Verify that two Siafu checkouts have the same tracked content.
#
# The public repository uses github.com/gac982/siafu and the private mirror
# uses github.com/proyect-octopus/siafu, so the module paths are normalized
# before diffing. The .git histories are intentionally independent.
#
# Usage: scripts/check-parity.sh /path/to/the/other/checkout

set -euo pipefail

if [ "$#" -ne 1 ]; then
	echo "uso: $0 <ruta-del-otro-repo>" >&2
	exit 2
fi

here="$(cd "$(dirname "$0")/.." && pwd)"
other="$(cd "$1" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

copy_tree() {
	local repo="$1"
	local dest="$2"
	mkdir -p "$dest"
	git -C "$repo" archive HEAD | tar -x -C "$dest"
	while IFS= read -r file; do
		sed \
			-e 's#github.com/gac982/siafu#MODULE#g' \
			-e 's#github.com/proyect-octopus/siafu#MODULE#g' \
			"$file" >"$file.tmp"
		mv "$file.tmp" "$file"
	done < <(grep -rl -e 'github.com/gac982/siafu' -e 'github.com/proyect-octopus/siafu' "$dest" || true)
}

copy_tree "$here" "$tmp/here"
copy_tree "$other" "$tmp/other"

if diff -rq "$tmp/here" "$tmp/other"; then
	echo "paridad OK: $here y $other tienen el mismo contenido"
else
	echo "paridad ROTA: revisa las diferencias anteriores" >&2
	exit 1
fi
