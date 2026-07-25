import { NavLink, Outlet } from "react-router-dom";

import { MoonIcon, SunIcon } from "@/components/icons";
import { useTheme } from "@/components/theme-provider";

const navItems = [
  { to: "/", label: "Home" },
  { to: "/blog", label: "Blog" },
  { to: "/resume", label: "Resume" },
] as const;

function navLinkClass({ isActive }: { isActive: boolean }): string {
  const base =
    "font-mono text-xs uppercase tracking-[0.2em] transition-[transform,color] active:translate-y-0.5 md:[writing-mode:vertical-rl]";
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
      className="rounded-chip border border-ink-200 p-1.5 text-ink-600 transition-[transform,color] hover:text-ink-900 active:translate-y-0.5 dark:border-ink-800 dark:text-ink-300 dark:hover:text-paper"
    >
      {theme === "dark" ? <SunIcon className="size-4" /> : <MoonIcon className="size-4" />}
    </button>
  );
}

/**
 * Left vertical rail on desktop: monogram up top, nav links bottom-aligned
 * and rotated vertical, theme toggle underneath. Collapses to a top bar on
 * mobile.
 */
function Rail() {
  return (
    <header className="fixed inset-x-0 top-0 z-10 flex h-14 items-center justify-between border-b border-ink-200 bg-paper px-5 md:inset-y-0 md:right-auto md:h-auto md:w-16 md:flex-col md:border-r md:border-b-0 md:px-0 md:py-7 dark:border-ink-800 dark:bg-ink-950">
      <NavLink
        to="/"
        className="font-display text-lg font-semibold tracking-tight transition-transform active:translate-y-0.5"
      >
        YCC
      </NavLink>
      <nav aria-label="Primary" className="flex items-center gap-5 md:mt-auto md:mb-7 md:flex-col md:gap-6">
        {navItems.map((item) => (
          <NavLink key={item.to} to={item.to} className={navLinkClass}>
            {item.label}
          </NavLink>
        ))}
      </nav>
      <ThemeToggle />
    </header>
  );
}

export function Layout() {
  return (
    <div className="min-h-dvh bg-paper font-sans text-ink-900 dark:bg-ink-950 dark:text-ink-200">
      <Rail />
      {/* Deliberately off-center: generous left offset, tighter right margin. */}
      <main className="px-5 pt-24 pb-24 md:pr-8 md:pl-36 lg:pl-48">
        <div className="max-w-2xl">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
