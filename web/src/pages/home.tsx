import { Link } from "react-router-dom";

import { AccordionItem } from "@/components/accordion";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { Contact, HomeBody, HomeDomain, Page, PublicSettings } from "@/types/cms";

const linkClass =
  "inline-block font-mono text-xs uppercase tracking-[0.18em] text-ink-600 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper";

const quietLinkClass =
  "mt-3 inline-block font-mono text-xs tracking-[0.14em] text-ink-500 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-500 dark:hover:text-paper";

function parseHomeBody(raw: string): HomeBody {
  try {
    const parsed = JSON.parse(raw) as HomeBody;
    return {
      eyebrow: parsed.eyebrow ?? "",
      headline: parsed.headline ?? "",
      intro: parsed.intro ?? "",
      domains: parsed.domains ?? [],
      now: parsed.now ?? "",
      accordion: Boolean(parsed.accordion),
    };
  } catch {
    return { eyebrow: "", headline: "", intro: "", domains: [], now: "", accordion: false };
  }
}

function DomainBody({ domain }: { domain: HomeDomain }) {
  return (
    <>
      <p className="text-base leading-relaxed text-ink-600 dark:text-ink-300">{domain.blurb}</p>
      {domain.link ? (
        <Link to={domain.link.to} className={quietLinkClass}>
          {domain.link.label}
        </Link>
      ) : null}
    </>
  );
}

export function HomePage() {
  const page = useApi<Page>("/api/pages/home");
  const settings = useApi<PublicSettings>("/api/settings");

  const body = page.data ? parseHomeBody(page.data.body_json) : null;
  const title = page.data?.title || "Home";
  const desc = page.data?.meta_description || settings.data?.meta_description || "";

  useDocumentMeta(title, desc);

  if (page.loading || settings.loading) return <LoadingState />;
  if (page.error) return <ErrorState message={page.error.message} />;
  if (!body || (!body.headline && !body.intro)) {
    return <EmptyState message="Home page content has not been published yet." />;
  }

  const contact: Contact = settings.data?.contact ?? { email: "", github: "", linkedin: "" };

  return (
    <article>
      {body.eyebrow ? (
        <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
          {body.eyebrow}
        </p>
      ) : null}
      {body.headline ? (
        <h1 className="mt-5 font-display text-4xl leading-tight font-semibold md:text-5xl">
          {body.headline}
        </h1>
      ) : null}
      {body.intro ? (
        <p className="mt-8 max-w-xl text-lg leading-relaxed text-ink-600 dark:text-ink-300">
          {body.intro}
        </p>
      ) : null}

      {body.domains.length > 0 ? (
        <section className="mt-20" aria-labelledby="domains-heading">
          <h2
            id="domains-heading"
            className="font-mono text-xs tracking-[0.22em] text-ink-600 uppercase dark:text-ink-400"
          >
            Focus
          </h2>
          {body.accordion ? (
            <div className="mt-4 max-w-lg">
              {body.domains.map((d) => (
                <AccordionItem
                  key={d.title}
                  id={`home-domain-${d.title}`}
                  className={d.offset ?? ""}
                  summary={
                    <h3 className="font-display text-xl font-semibold tracking-tight">{d.title}</h3>
                  }
                >
                  <DomainBody domain={d} />
                </AccordionItem>
              ))}
            </div>
          ) : (
            <ul className="mt-8 space-y-12">
              {body.domains.map((d) => (
                <li key={d.title} className={`max-w-lg ${d.offset ?? ""}`}>
                  <h3 className="font-display text-xl font-semibold tracking-tight">{d.title}</h3>
                  <div className="mt-2">
                    <DomainBody domain={d} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      ) : null}

      {body.now ? (
        <p className="mt-20 max-w-md border-t border-ink-200 pt-8 font-mono text-sm leading-relaxed text-ink-600 dark:border-ink-800 dark:text-ink-400">
          {body.now}
        </p>
      ) : null}

      <footer className="mt-16 flex flex-wrap gap-x-6 gap-y-3">
        <Link to="/resume" className={linkClass}>
          Resume
        </Link>
        <Link to="/blog" className={linkClass}>
          Notes
        </Link>
        <Link to="/work" className={linkClass}>
          Work
        </Link>
        {contact.github ? (
          <a href={contact.github} target="_blank" rel="noreferrer" className={linkClass}>
            GitHub
          </a>
        ) : null}
        {contact.email ? (
          <a href={`mailto:${contact.email}`} className={linkClass}>
            Email
          </a>
        ) : null}
        {contact.linkedin ? (
          <a href={contact.linkedin} target="_blank" rel="noreferrer" className={linkClass}>
            LinkedIn
          </a>
        ) : null}
      </footer>
    </article>
  );
}
