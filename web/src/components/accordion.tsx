import type { ReactNode } from "react";

import { ChevronIcon } from "@/components/icons";

type AccordionItemProps = {
  /** Stable id for aria / fragment anchors. */
  id: string;
  /** Visible summary row (title / meta). */
  summary: ReactNode;
  /** Collapsed body. */
  children: ReactNode;
  className?: string;
};

/**
 * Single disclosure row. Uses native details/summary for keyboard and
 * screen-reader support; styling matches the site rail/press language.
 */
export function AccordionItem({ id, summary, children, className = "" }: AccordionItemProps) {
  return (
    <details
      id={id}
      className={`group border-t border-ink-200 first:border-t-0 dark:border-ink-800 ${className}`}
    >
      <summary className="flex cursor-pointer list-none items-start justify-between gap-3 py-4 transition-transform marker:content-none active:translate-y-0.5 [&::-webkit-details-marker]:hidden">
        <div className="min-w-0 flex-1">{summary}</div>
        <ChevronIcon className="mt-1 size-4 shrink-0 text-ink-500 transition-transform duration-200 group-open:rotate-180 dark:text-ink-400" />
      </summary>
      <div className="pb-5">{children}</div>
    </details>
  );
}
