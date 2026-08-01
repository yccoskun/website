import { AccordionItem } from "@/components/accordion";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { useDocumentMeta } from "@/lib/meta";
import { isValidEmail, safeHref } from "@/lib/urls";
import { useApi } from "@/lib/use-api";
import type { Contact, PublicSettings } from "@/types/cms";
import type { Resume, ResumeEntry, ResumeHeader } from "@/types/resume";

const linkClass =
  "inline-block font-mono text-xs uppercase tracking-[0.18em] text-ink-600 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper";

const primaryCtaClass =
  "inline-block rounded-chip border border-ink-900 bg-ink-900 px-3 py-1.5 font-mono text-xs tracking-[0.14em] text-paper uppercase transition-transform active:translate-y-0.5 dark:border-paper dark:bg-paper dark:text-ink-950";

function splitTech(tech: string): string[] {
  return tech
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
}

function entryHeading(entry: ResumeEntry): string {
  return [entry.role, entry.org].filter(Boolean).join(" · ");
}

function entryMeta(entry: ResumeEntry): string {
  return [entry.period, entry.location].filter(Boolean).join(" · ");
}

function ResumeEntryDetail({ entry }: { entry: ResumeEntry }) {
  const chips = splitTech(entry.tech);
  return (
    <>
      {entry.body_html ? (
        <div
          className="prose-site text-[0.95rem]"
          dangerouslySetInnerHTML={{ __html: entry.body_html }}
        />
      ) : null}
      {chips.length > 0 ? (
        <ul className="mt-4 flex flex-wrap gap-1.5">
          {chips.map((chip) => (
            <li
              key={chip}
              className="rounded-chip border border-ink-200 px-1.5 py-0.5 font-mono text-[0.7rem] text-ink-600 dark:border-ink-800 dark:text-ink-400"
            >
              {chip}
            </li>
          ))}
        </ul>
      ) : null}
    </>
  );
}

function ResumeEntryBlock({ entry, accordion }: { entry: ResumeEntry; accordion: boolean }) {
  const heading = entryHeading(entry);
  const meta = entryMeta(entry);
  const hasDetail = Boolean(entry.body_html) || splitTech(entry.tech).length > 0;

  if (accordion && hasDetail) {
    return (
      <AccordionItem
        id={`resume-entry-${entry.id}`}
        summary={
          <div>
            {heading ? (
              <h3 className="font-display text-xl font-semibold tracking-tight">{heading}</h3>
            ) : (
              <h3 className="font-display text-xl font-semibold tracking-tight">Entry</h3>
            )}
            {meta ? (
              <p className="mt-1.5 font-mono text-xs tracking-wide text-ink-600 dark:text-ink-400">
                {meta}
              </p>
            ) : null}
          </div>
        }
      >
        <ResumeEntryDetail entry={entry} />
      </AccordionItem>
    );
  }

  return (
    <li className="border-t border-ink-200 pt-8 first:border-t-0 first:pt-0 dark:border-ink-800">
      {heading ? (
        <h3 className="font-display text-xl font-semibold tracking-tight">{heading}</h3>
      ) : null}
      {meta ? (
        <p className="mt-1.5 font-mono text-xs tracking-wide text-ink-600 dark:text-ink-400">
          {meta}
        </p>
      ) : null}
      {hasDetail ? (
        <div className="mt-4">
          <ResumeEntryDetail entry={entry} />
        </div>
      ) : null}
    </li>
  );
}

export function ResumePage() {
  const { data, loading, error } = useApi<Resume>("/api/resume");
  const settings = useApi<PublicSettings>("/api/settings");

  const header = data?.header;
  useDocumentMeta(
    header?.headline || "Resume",
    header?.blurb || settings.data?.meta_description || "",
  );

  if (loading) {
    return (
      <article>
        <ResumeHeaderBlock header={undefined} contact={undefined} />
        <div className="mt-12">
          <LoadingState />
        </div>
      </article>
    );
  }

  if (error) {
    return (
      <article>
        <ResumeHeaderBlock header={header} contact={settings.data?.contact} />
        <div className="mt-12">
          <ErrorState message={error.message} />
        </div>
      </article>
    );
  }

  const sections = (data?.sections ?? []).filter((s) => s.entries.length > 0);
  if (sections.length === 0) {
    return (
      <article>
        <ResumeHeaderBlock header={header} contact={settings.data?.contact} />
        <div className="mt-12">
          <EmptyState message="No resume entries yet." />
        </div>
      </article>
    );
  }

  return (
    <article>
      <ResumeHeaderBlock header={header} contact={settings.data?.contact} />
      <div className="mt-16 space-y-16">
        {sections.map((section) => (
          <section key={section.id} aria-labelledby={`resume-sec-${section.id}`}>
            <h2
              id={`resume-sec-${section.id}`}
              className="font-mono text-xs tracking-[0.22em] text-ink-600 uppercase dark:text-ink-400"
            >
              {section.title}
            </h2>
            {section.accordion ? (
              <div className="mt-4">
                {section.entries.map((entry) => (
                  <ResumeEntryBlock key={entry.id} entry={entry} accordion />
                ))}
              </div>
            ) : (
              <ul className="mt-8 space-y-10">
                {section.entries.map((entry) => (
                  <ResumeEntryBlock key={entry.id} entry={entry} accordion={false} />
                ))}
              </ul>
            )}
          </section>
        ))}
      </div>
    </article>
  );
}

function ResumeHeaderBlock({
  header,
  contact,
}: {
  header: ResumeHeader | undefined;
  contact: Contact | undefined;
}) {
  if (!header?.headline && !header?.blurb) {
    return (
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        Resume
      </p>
    );
  }

  const github = safeHref(contact?.github);
  const linkedin = safeHref(contact?.linkedin);
  const emailOk = Boolean(contact?.email?.trim()) && isValidEmail(contact?.email ?? "");

  return (
    <>
      {header.eyebrow ? (
        <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
          {header.eyebrow}
        </p>
      ) : null}
      {header.headline ? (
        <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">{header.headline}</h1>
      ) : null}
      {header.blurb ? (
        <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-600 dark:text-ink-300">
          {header.blurb}
        </p>
      ) : null}

      <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-3">
        {header.pdf_url ? (
          <a href={header.pdf_url} download="YusufCan-Coskun-CV.pdf" className={primaryCtaClass}>
            Download PDF
          </a>
        ) : null}
        {linkedin ? (
          <a href={linkedin} target="_blank" rel="noreferrer" className={linkClass}>
            LinkedIn
          </a>
        ) : null}
        {github ? (
          <a href={github} target="_blank" rel="noreferrer" className={linkClass}>
            GitHub
          </a>
        ) : null}
        {emailOk && contact ? (
          <a href={`mailto:${contact.email.trim()}`} className={linkClass}>
            Email
          </a>
        ) : null}
      </div>
    </>
  );
}
