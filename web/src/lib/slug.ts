const POST_SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const MAX_POST_SLUG_LEN = 100;

/** Matches server post slug rules: trimmed kebab of [a-z0-9], length 1–100. */
export function isValidPostSlug(slug: string): boolean {
  const trimmed = slug.trim();
  if (!trimmed || trimmed.length > MAX_POST_SLUG_LEN) return false;
  return POST_SLUG_PATTERN.test(trimmed);
}

/** Turkish-aware lowercase slug: transliterate, hyphenate, never empty for non-empty titles. */
export function slugify(title: string): string {
  const trimmed = title.trim();
  if (!trimmed) return "";

  const transliterated = trimmed
    .replace(/ı/g, "i")
    .replace(/İ/g, "i")
    .replace(/I/g, "i")
    .replace(/ş/g, "s")
    .replace(/Ş/g, "s")
    .replace(/ğ/g, "g")
    .replace(/Ğ/g, "g")
    .replace(/ü/g, "u")
    .replace(/Ü/g, "u")
    .replace(/ö/g, "o")
    .replace(/Ö/g, "o")
    .replace(/ç/g, "c")
    .replace(/Ç/g, "c");

  const slug = transliterated
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  return slug || "post";
}
