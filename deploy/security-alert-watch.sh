#!/usr/bin/env bash
# Follow website journal lines that include alert=1 (structured security alerts).
# Optional: set WEBHOOK_URL to POST each matching line as text/plain.
# Optional: set UNIT (default: website) for the systemd unit name.
set -euo pipefail

UNIT="${UNIT:-website}"

journalctl -u "$UNIT" -f -o cat | grep --line-buffered 'alert=1' | while IFS= read -r line; do
	printf '%s\n' "$line"
	if [[ -n "${WEBHOOK_URL:-}" ]]; then
		if ! curl -fsS -X POST \
			-H 'Content-Type: text/plain; charset=utf-8' \
			--data-binary "$line" \
			"$WEBHOOK_URL" >/dev/null; then
			printf 'security-alert-watch: webhook POST failed\n' >&2
		fi
	fi
done
