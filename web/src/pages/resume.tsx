import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { Resume, ResumeEntry } from "@/types/resume";

function splitTech(tech: string): string[] {
  return tech
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
}

function ResumeEntryBlock({ entry }: { entry: ResumeEntry }) {
  const chips = splitTech(entry.tech);
  const heading = [entry.role, entry.org].filter(Boolean).join(" · ");
  const meta = [entry.period, entry.location].filter(Boolean).join(" · ");

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
      {entry.body_html ? (
        <div
          className="prose-site mt-4 text-[0.95rem]"
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
    </li>
  );
}

export function ResumePage() {
  const { data, loading, error } = useApi<Resume>("/api/resume");

  useDocumentMeta("Resume", "Resume of Yusuf Can Coskun — placeholder entries for now.");

  if (loading) {
    return (
      <article>
        <ResumeHeader />
        <div className="mt-12">
          <LoadingState />
        </div>
      </article>
    );
  }

  if (error) {
    return (
      <article>
        <ResumeHeader />
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
        <ResumeHeader />
        <div className="mt-12">
          <EmptyState message="No resume entries yet." />
        </div>
      </article>
    );
  }

  return (
    <article>
      <ResumeHeader />
      <div className="mt-16 space-y-16">
        {sections.map((section) => (
          <section key={section.id} aria-labelledby={`resume-sec-${section.id}`}>
            <h2
              id={`resume-sec-${section.id}`}
              className="font-mono text-xs tracking-[0.22em] text-ink-600 uppercase dark:text-ink-400"
            >
              {section.title}
            </h2>
            <ul className="mt-8 space-y-10">
              {section.entries.map((entry) => (
                <ResumeEntryBlock key={entry.id} entry={entry} />
              ))}
            </ul>
          </section>
        ))}
      </div>
    </article>
  );
}

function ResumeHeader() {
  return (
    <>
      <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
        Resume
      </p>
      <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">
        Placeholder resume — real entries coming soon.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-600 dark:text-ink-300">
        Everything below is placeholder data seeded at first run. The real career history
        replaces it through the admin panel.
      </p>
    </>
  );
}
