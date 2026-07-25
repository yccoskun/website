/** Quiet mono loading line — no spinner, no shimmer. */
export function LoadingState() {
  return (
    <p
      role="status"
      aria-live="polite"
      className="font-mono text-sm text-ink-600 dark:text-ink-400"
    >
      Loading…
    </p>
  );
}

/** Honest empty copy for lists or sections with nothing to show. */
export function EmptyState({ message }: { message: string }) {
  return <p className="font-mono text-sm text-ink-600 dark:text-ink-400">{message}</p>;
}

/** Error line using the ember accent sparingly. */
export function ErrorState({ message }: { message: string }) {
  return (
    <p
      role="status"
      aria-live="polite"
      className="font-mono text-sm text-ember-600 dark:text-ember-400"
    >
      {message}
    </p>
  );
}
