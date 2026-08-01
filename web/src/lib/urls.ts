const NAV_PATH_RE = /^[A-Za-z0-9/_#?&=%.~+-]+$/;

/** True when raw is empty (if allowed) or an https URL. Rejects javascript:/data:/vbscript:/ //. */
export function isValidHTTPSURL(raw: string, allowEmpty = false): boolean {
  const s = raw.trim();
  if (!s) return allowEmpty;
  if (s.startsWith("//")) return false;
  const lower = s.toLowerCase();
  if (
    lower.startsWith("javascript:") ||
    lower.startsWith("data:") ||
    lower.startsWith("vbscript:")
  ) {
    return false;
  }
  try {
    const u = new URL(s);
    return u.protocol.toLowerCase() === "https:";
  } catch {
    return false;
  }
}

/** True when raw is a relative path starting with a single /. */
export function isValidNavPath(raw: string): boolean {
  const s = raw.trim();
  if (!s || s.startsWith("//") || !s.startsWith("/")) return false;
  if (s.includes("://")) return false;
  return NAV_PATH_RE.test(s);
}

/** Empty is valid. Non-empty must contain @ and no whitespace, colon, or control chars. */
export function isValidEmail(raw: string): boolean {
  const s = raw.trim();
  if (!s) return true;
  if (s.startsWith("//")) return false;
  if (!s.includes("@")) return false;
  for (const ch of s) {
    const code = ch.charCodeAt(0);
    if (ch === ":" || code <= 32 || code === 127) return false;
  }
  return true;
}

/** Returns a trimmed https URL, or undefined when unsafe/empty. */
export function safeHref(raw: string | null | undefined): string | undefined {
  if (raw == null) return undefined;
  const s = raw.trim();
  if (!s || !isValidHTTPSURL(s, false)) return undefined;
  return s;
}

/** Returns a trimmed nav path, or undefined when unsafe. */
export function safePath(raw: string | null | undefined): string | undefined {
  if (raw == null) return undefined;
  const s = raw.trim();
  if (!isValidNavPath(s)) return undefined;
  return s;
}
