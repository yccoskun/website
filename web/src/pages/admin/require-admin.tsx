import type { ReactNode } from "react";

import { ErrorState, LoadingState } from "@/components/states";
import { LoginPage } from "@/pages/admin/login";
import { useAdminSession } from "@/pages/admin/session";

/** Gates admin routes: checking → load; error → retry; anonymous → login; authed → children. */
export function RequireAdmin({ children }: { children: ReactNode }) {
  const { status, bootError, retry } = useAdminSession();

  if (status === "checking") {
    return (
      <div className="min-h-dvh bg-paper px-5 pt-16 font-sans dark:bg-ink-950">
        <LoadingState />
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className="min-h-dvh bg-paper px-5 pt-16 font-sans dark:bg-ink-950">
        <ErrorState message={bootError ?? "Session check failed"} />
        <button
          type="button"
          onClick={retry}
          className="mt-3 rounded-chip border border-ink-200 px-2.5 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-ink-600 uppercase transition-transform hover:text-ink-900 active:translate-y-0.5 dark:border-ink-800 dark:text-ink-400 dark:hover:text-paper"
        >
          Retry
        </button>
      </div>
    );
  }

  if (status === "anonymous") {
    return <LoginPage />;
  }

  return <>{children}</>;
}
