import { Link } from "react-router-dom";

import { useDocumentMeta } from "@/lib/meta";

const linkClass =
  "inline-block font-mono text-xs uppercase tracking-[0.18em] text-ink-600 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper";

// Placeholder sections — real domain write-ups replace these before launch copy is final.
const domains = [
  {
    title: "First focus area",
    blurb:
      "Placeholder paragraph. A few sentences about one area of work will live here — what it is, why it matters, and what shipped.",
    offset: "",
  },
  {
    title: "Second focus area",
    blurb:
      "Placeholder paragraph. Another short block of text holding the layout in place until the real story is written.",
    offset: "md:ml-10 lg:ml-16",
  },
  {
    title: "Third focus area",
    blurb:
      "Placeholder paragraph. Same deal — this text only exists so the page reads like a page while the content catches up.",
    offset: "md:ml-4 lg:ml-8 max-w-md",
  },
] as const;

export function HomePage() {
  useDocumentMeta(
    "Home",
    "Personal website of Yusuf Can Coskun — placeholder content while the real thing is being written.",
  );

  return (
    <article>
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        Yusuf Can Coskun — software engineer
      </p>
      <h1 className="mt-5 font-display text-4xl leading-tight font-semibold md:text-5xl">
        Placeholder headline — the real introduction is on its way.
      </h1>
      <p className="mt-8 max-w-xl text-lg leading-relaxed text-ink-600 dark:text-ink-300">
        This site is live before the words are. Everything on this page is placeholder copy
        keeping the layout honest; the real introduction lands here soon.
      </p>

      <section className="mt-20" aria-labelledby="domains-heading">
        <h2
          id="domains-heading"
          className="font-mono text-xs tracking-[0.22em] text-ink-600 uppercase dark:text-ink-400"
        >
          Three placeholders
        </h2>
        <ul className="mt-8 space-y-12">
          {domains.map((d) => (
            <li key={d.title} className={`max-w-lg ${d.offset}`}>
              <h3 className="font-display text-xl font-semibold tracking-tight">{d.title}</h3>
              <p className="mt-2 text-base leading-relaxed text-ink-600 dark:text-ink-300">
                {d.blurb}
              </p>
            </li>
          ))}
        </ul>
      </section>

      <p className="mt-20 max-w-md border-t border-ink-200 pt-8 font-mono text-sm leading-relaxed text-ink-600 dark:border-ink-800 dark:text-ink-400">
        Now: replacing this placeholder text with the real content, one page at a time.
      </p>

      <footer className="mt-16 flex flex-wrap gap-x-6 gap-y-3">
        <Link to="/resume" className={linkClass}>
          Resume
        </Link>
        <Link to="/blog" className={linkClass}>
          Blog
        </Link>
        <a
          href="https://github.com/yccoskun"
          target="_blank"
          rel="noreferrer"
          className={linkClass}
        >
          GitHub
        </a>
        <a href="mailto:yusufcancoskun@gmail.com" className={linkClass}>
          Email
        </a>
      </footer>
    </article>
  );
}
