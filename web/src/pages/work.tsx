import { AccordionItem } from "@/components/accordion";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { useDocumentMeta } from "@/lib/meta";
import { useApi } from "@/lib/use-api";
import type { Page, WorkItem, WorkPageBody } from "@/types/cms";

const linkClass =
  "inline-block font-mono text-xs uppercase tracking-[0.18em] text-ink-600 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:text-ink-400 dark:hover:text-paper";

function parseWorkBody(raw: string): WorkPageBody {
  try {
    const parsed = JSON.parse(raw) as WorkPageBody;
    return {
      eyebrow: parsed.eyebrow ?? "Work",
      headline: parsed.headline ?? "",
      intro: parsed.intro ?? "",
      empty_message: parsed.empty_message || "Nothing listed yet.",
      accordion: Boolean(parsed.accordion),
    };
  } catch {
    return {
      eyebrow: "Work",
      headline: "",
      intro: "",
      empty_message: "Nothing listed yet.",
      accordion: false,
    };
  }
}

function WorkItemSummary({ item }: { item: WorkItem }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
      <h2 className="font-display text-2xl font-semibold tracking-tight">{item.name}</h2>
      {item.status ? (
        <span className="font-mono text-[0.7rem] tracking-[0.14em] text-ink-500 uppercase dark:text-ink-500">
          {item.status}
        </span>
      ) : null}
    </div>
  );
}

function WorkItemDetail({ item }: { item: WorkItem }) {
  return (
    <>
      {item.one_liner ? (
        <p className="text-base leading-relaxed text-ink-700 dark:text-ink-300">{item.one_liner}</p>
      ) : null}
      {item.body ? (
        <p className="mt-3 text-base leading-relaxed text-ink-600 dark:text-ink-400">{item.body}</p>
      ) : null}
      {item.stack.length > 0 ? (
        <ul className="mt-4 flex flex-wrap gap-1.5">
          {item.stack.map((chip) => (
            <li
              key={chip}
              className="rounded-chip border border-ink-200 px-1.5 py-0.5 font-mono text-[0.7rem] text-ink-600 dark:border-ink-800 dark:text-ink-400"
            >
              {chip}
            </li>
          ))}
        </ul>
      ) : null}
      {item.href ? (
        <a href={item.href} target="_blank" rel="noreferrer" className={`${linkClass} mt-5`}>
          {item.name} on GitHub
        </a>
      ) : null}
    </>
  );
}

export function WorkPage() {
  const page = useApi<Page>("/api/pages/work");
  const items = useApi<WorkItem[]>("/api/work");

  const body = page.data ? parseWorkBody(page.data.body_json) : null;

  useDocumentMeta(page.data?.title || "Work", page.data?.meta_description || "");

  if (page.loading || items.loading) return <LoadingState />;
  if (page.error) return <ErrorState message={page.error.message} />;
  if (items.error) return <ErrorState message={items.error.message} />;
  if (!body) return <EmptyState message="Work page content has not been published yet." />;

  const list = items.data ?? [];

  return (
    <article>
      {body.eyebrow ? (
        <p className="font-mono text-xs tracking-[0.25em] text-ink-600 uppercase dark:text-ink-400">
          {body.eyebrow}
        </p>
      ) : null}
      {body.headline ? (
        <h1 className="mt-5 font-display text-4xl leading-tight font-semibold">{body.headline}</h1>
      ) : null}
      {body.intro ? (
        <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-600 dark:text-ink-300">
          {body.intro}
        </p>
      ) : null}

      {list.length === 0 ? (
        <p className="mt-14 font-mono text-sm text-ink-600 dark:text-ink-400">{body.empty_message}</p>
      ) : body.accordion ? (
        <div className="mt-10 max-w-lg">
          {list.map((item, i) => (
            <AccordionItem
              key={item.id}
              id={`work-${item.id}`}
              className={i % 2 === 1 ? "md:ml-8" : ""}
              summary={<WorkItemSummary item={item} />}
            >
              <WorkItemDetail item={item} />
            </AccordionItem>
          ))}
        </div>
      ) : (
        <ul className="mt-14 space-y-14">
          {list.map((item, i) => (
            <li key={item.id} className={`max-w-lg ${i % 2 === 1 ? "md:ml-8" : ""}`}>
              <WorkItemSummary item={item} />
              <div className="mt-2">
                <WorkItemDetail item={item} />
              </div>
            </li>
          ))}
        </ul>
      )}
    </article>
  );
}
