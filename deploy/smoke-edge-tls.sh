#!/usr/bin/env bash
# Probe live Cloudflare edge for HSTS, HTTP→HTTPS redirects, and mixed-content
# asset URLs. Production-only — not for CI or localhost. Fails until edge
# SSL/TLS settings (Always Use HTTPS + HSTS) are applied in the Cloudflare dashboard.
#
# Mixed-content check is HTML-attribute-only (quoted src=/href=). It does not
# cover srcset, CSS url(), or JS-injected URLs.
set -euo pipefail

HOSTS=(www.yusufcancoskun.com yusufcancoskun.com)
MIN_MAX_AGE=15552000
HEALTH_PATH=/api/health
ADMIN_PATH=/admin

failures=0

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; failures=$((failures + 1)); }

# Extract max-age from a Strict-Transport-Security header value (case-insensitive).
# Allows optional whitespace around '=' (RFC 6797 OWS). Prints integer or empty.
hsts_max_age() {
	local header="$1"
	local value
	value="$(printf '%s' "$header" | tr '[:upper:]' '[:lower:]')"
	if [[ "$value" =~ max-age[[:space:]]*=[[:space:]]*([0-9]+) ]]; then
		printf '%s' "${BASH_REMATCH[1]}"
	fi
}

# True if hostname is one of the expected apex/www hosts.
allowed_location_host() {
	local host="$1"
	local h
	for h in "${HOSTS[@]}"; do
		if [[ "$host" == "$h" ]]; then
			return 0
		fi
	done
	return 1
}

# Last HTTP status line that is not 1xx (skips Early Hints / informational).
final_http_status() {
	local headers="$1"
	local line code last=""
	while IFS= read -r line; do
		if [[ "$line" =~ ^HTTP/ ]]; then
			code="$(printf '%s' "$line" | awk '{print $2}')"
			if [[ "$code" =~ ^[0-9]{3}$ ]] && (( 10#$code < 100 || 10#$code >= 200 )); then
				last="$code"
			fi
		fi
	done < <(printf '%s' "$headers" | tr -d '\r')
	printf '%s' "$last"
}

check_hsts() {
	local host="$1"
	local url="https://${host}${HEALTH_PATH}"
	local headers hsts_line max_age

	headers="$(curl -fsSI --max-time 20 "$url" 2>&1)" || {
		fail "HSTS ${host}: HTTPS probe failed (${url})"
		return
	}

	hsts_line="$(printf '%s' "$headers" | tr -d '\r' | grep -i '^strict-transport-security:' | head -n1 || true)"
	if [[ -z "$hsts_line" ]]; then
		fail "HSTS ${host}: Strict-Transport-Security header missing"
		return
	fi

	max_age="$(hsts_max_age "${hsts_line#*:}")"
	if [[ -z "$max_age" ]]; then
		fail "HSTS ${host}: could not parse max-age from: ${hsts_line}"
		return
	fi
	if (( max_age < MIN_MAX_AGE )); then
		fail "HSTS ${host}: max-age=${max_age} < ${MIN_MAX_AGE}"
		return
	fi

	pass "HSTS ${host}: max-age=${max_age}"
}

check_http_redirect() {
	local host="$1"
	local url="http://${host}${HEALTH_PATH}"
	local headers status location loc_host

	# Do not follow redirects; we assert the first non-1xx hop is HTTPS.
	headers="$(curl -sSI --max-time 20 "$url" 2>&1)" || {
		fail "HTTP redirect ${host}: probe failed (${url})"
		return
	}

	status="$(final_http_status "$headers")"
	location="$(printf '%s' "$headers" | tr -d '\r' | grep -i '^location:' | tail -n1 | sed 's/^[Ll]ocation:[[:space:]]*//' || true)"

	if [[ ! "$status" =~ ^3[0-9][0-9]$ ]]; then
		fail "HTTP redirect ${host}: expected 3xx, got ${status:-unknown}"
		return
	fi
	# 304 is not a redirect.
	if [[ "$status" == "304" ]]; then
		fail "HTTP redirect ${host}: got 304 (not a redirect)"
		return
	fi
	if [[ -z "$location" ]]; then
		fail "HTTP redirect ${host}: 3xx without Location header"
		return
	fi
	if [[ ! "$location" =~ ^https:// ]]; then
		fail "HTTP redirect ${host}: Location is not https:// (${location})"
		return
	fi

	loc_host="$(printf '%s' "$location" | sed -E 's|^https://([^/]+).*|\1|' | tr '[:upper:]' '[:lower:]')"
	# Strip optional :443
	loc_host="${loc_host%:443}"
	if ! allowed_location_host "$loc_host"; then
		fail "HTTP redirect ${host}: Location host not apex/www (${location})"
		return
	fi

	pass "HTTP redirect ${host}: ${status} → ${location}"
}

check_admin_assets() {
	local host="$1"
	local url="https://${host}${ADMIN_PATH}"
	local html bad

	html="$(curl -fsS --max-time 20 "$url" 2>&1)" || {
		fail "Admin assets ${host}: fetch failed (${url})"
		return
	}

	# Absolute http:// in quoted src= / href= only (ignore https and protocol-relative).
	bad="$(printf '%s' "$html" | grep -Eoi '(src|href)=["'\'']http://[^"'\'']+["'\'']' || true)"
	if [[ -n "$bad" ]]; then
		fail "Admin assets ${host}: absolute http:// asset URL(s) found:"
		printf '%s\n' "$bad" | sed 's/^/  /'
		return
	fi

	pass "Admin assets ${host}: no absolute http:// src/href"
}

for host in "${HOSTS[@]}"; do
	check_hsts "$host"
	check_http_redirect "$host"
	check_admin_assets "$host"
done

if (( failures > 0 )); then
	printf '\n%d check(s) failed. Apply Cloudflare Always Use HTTPS + HSTS (see deploy/README.md).\n' "$failures" >&2
	exit 1
fi

printf '\nAll edge TLS / HSTS checks passed.\n'
