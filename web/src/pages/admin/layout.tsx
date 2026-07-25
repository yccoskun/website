import { NavLink, Outlet } from "react-router-dom";

import { MoonIcon, SunIcon } from "@/components/icons";
import { useTheme } from "@/components/theme-provider";
import { useAdminSession } from "@/pages/admin/session";

const navItems = [
  { to: "/admin/posts", label: "Posts", end: false },
  { to: "/admin/resume", label: "Resume", end: true },
] as const;

function navClass({ isActive }: { isActive: boolean }): string {
  const base =
    "rounded-chip px-2 py-1 font-mono text-xs tracking-[0.16em] uppercase transition-[transform,color] active:translate-y-0.5";
  if (isActive) return `${base} text-ember-600 dark:text-ember-400`;
  return `${base} text-ink-600 hover:text-ink-900 dark:text-ink-400 dark:hover:text-paper`;
}

function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const next = theme === "dark" ? "light" : "dark";

  return (
    <button
      type="button"
      aria-label={`Switch to ${next} theme`}
      onClick={() => setTheme(next)}
      className="rounded-chip border border-ink-200 p-1 text-ink-600 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:border-ink-800 dark:text-ink-300 dark:hover:text-paper"
    >
      {theme === "dark" ? <SunIcon className="size-3.5" /> : <MoonIcon className="size-3.5" />}
    </button>
  );
}

export function AdminLayout() {
  const { username, logout } = useAdminSession();

  return (
    <div className="min-h-dvh bg-paper font-sans text-ink-900 dark:bg-ink-950 dark:text-ink-200">
      {/* Mobile: top bar. md+: fixed dense sidebar. */}
      <aside className="fixed inset-x-0 top-0 z-10 flex h-14 items-center gap-3 border-b border-ink-200 bg-paper-raised px-3 md:inset-y-0 md:right-auto md:h-auto md:w-40 md:flex-col md:items-stretch md:gap-0 md:border-r md:border-b-0 md:px-3 md:py-4 lg:w-44 dark:border-ink-800 dark:bg-ink-900">
        <p className="shrink-0 font-display text-base font-semibold tracking-tight">YCC admin</p>
        <nav aria-label="Admin" className="flex flex-1 items-center gap-1 md:mt-6 md:flex-col md:items-stretch md:gap-0.5">
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={navClass}>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="flex shrink-0 items-center gap-2 md:mt-auto md:flex-col md:items-stretch md:gap-2 md:border-t md:border-ink-200 md:pt-3 dark:md:border-ink-800">
          <p className="hidden truncate font-mono text-[0.7rem] text-ink-600 md:block dark:text-ink-400">
            {username}
          </p>
          <div className="flex items-center gap-2">
            <span className="max-w-16 truncate font-mono text-[0.65rem] text-ink-600 md:hidden dark:text-ink-400">
              {username}
            </span>
            <button
              type="button"
              onClick={() => void logout()}
              className="rounded-chip border border-ink-200 px-2 py-1 font-mono text-[0.65rem] tracking-[0.14em] text-ink-600 uppercase transition-transform hover:text-ink-900 active:translate-y-0.5 dark:border-ink-800 dark:text-ink-400 dark:hover:text-paper"
            >
              Logout
            </button>
            <ThemeToggle />
          </div>
        </div>
      </aside>
      <main className="min-h-dvh pt-14 md:pt-0 md:pl-40 lg:pl-44">
        <div className="px-4 py-4 md:px-5 md:py-5">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
