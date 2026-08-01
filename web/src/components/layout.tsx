import { NavLink, Outlet } from "react-router";

import { MoonIcon, SunIcon } from "@/components/icons";
import { useTheme } from "@/components/theme-provider";
import { safePath } from "@/lib/urls";
import { useApi } from "@/lib/use-api";
import type { NavItem, PublicSettings } from "@/types/cms";

const fallbackNav: NavItem[] = [
  { label: "Home", path: "/" },
  { label: "Notes", path: "/blog" },
  { label: "Work", path: "/work" },
  { label: "Studio", path: "/studio" },
  { label: "Resume", path: "/resume" },
];

function navLinkClass({ isActive }: { isActive: boolean }): string {
  const base =
    "shrink-0 font-mono text-xs uppercase tracking-[0.18em] transition-[transform,color] active:translate-y-0.5 md:tracking-[0.2em] md:[writing-mode:vertical-rl]";
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
 * Mobile: horizontal bar — nav left, theme right.
 * Desktop: vertical rail — nav from the top, theme pinned to the bottom.
 */
function Rail({ settings }: { settings: PublicSettings | null }) {
  const nav = settings?.nav?.length ? settings.nav : fallbackNav;

  return (
    <header className="fixed inset-x-0 top-0 z-10 flex h-14 items-center gap-4 border-b border-ink-200 bg-paper px-5 md:inset-y-0 md:right-auto md:h-auto md:w-16 md:flex-col md:gap-0 md:border-r md:border-b-0 md:px-0 md:py-7 dark:border-ink-800 dark:bg-ink-950">
      {/* min-w-0 + hidden scrollbar: overflow scrolls inside the nav, not over Resume. */}
      <nav
        aria-label="Primary"
        className="flex min-w-0 flex-1 items-center gap-4 overflow-x-auto overscroll-x-contain [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden md:min-h-0 md:flex-none md:flex-col md:gap-6 md:overflow-visible"
      >
        {nav.map((item) => {
          const path = safePath(item.path);
          if (!path) {
            return (
              <span key={item.path} className={navLinkClass({ isActive: false })}>
                {item.label}
              </span>
            );
          }
          return (
            <NavLink key={path} to={path} className={navLinkClass} end={path === "/"}>
              {item.label}
            </NavLink>
          );
        })}
      </nav>
      <div className="shrink-0 md:mt-auto">
        <ThemeToggle />
      </div>
    </header>
  );
}

export function Layout() {
  const { data } = useApi<PublicSettings>("/api/settings");

  return (
    <div className="min-h-dvh bg-paper font-sans text-ink-900 dark:bg-ink-950 dark:text-ink-200">
      <Rail settings={data} />
      {/* Off-center column: generous left offset past the rail, still more air on the right. */}
      <main className="px-5 pt-24 pb-24 md:pr-24 md:pl-52 lg:pr-40 lg:pl-72 xl:pr-52 xl:pl-80">
        <div className="max-w-2xl">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
