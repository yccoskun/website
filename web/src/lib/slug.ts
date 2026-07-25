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
