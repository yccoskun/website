import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { ReactNode } from "react";
import { useLocation, useNavigate } from "react-router";

import { ApiError, apiGet, apiPost } from "@/lib/api";
import type { LoginOk, MeResponse } from "@/types/admin";

export type AdminSessionStatus = "checking" | "anonymous" | "authed" | "error";

interface AdminSessionValue {
  status: AdminSessionStatus;
  username: string | null;
  bootError: string | null;
  /** True when the last 401 was reauth_required (soft session binding). */
  reauthRequired: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  retry: () => void;
  onUnauthorized: (opts?: { reauth?: boolean }) => void;
}

const AdminSessionContext = createContext<AdminSessionValue | null>(null);

export function AdminSessionProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [status, setStatus] = useState<AdminSessionStatus>("checking");
  const [username, setUsername] = useState<string | null>(null);
  const [bootError, setBootError] = useState<string | null>(null);
  const [reauthRequired, setReauthRequired] = useState(false);
  const [retryToken, setRetryToken] = useState(0);

  /** Replace the current history entry to drop create-post seed from state. */
  const clearCurrentHistoryState = useCallback(() => {
    navigate(location.pathname || "/admin", { replace: true, state: null });
  }, [navigate, location.pathname]);

  useEffect(() => {
    let cancelled = false;
    setStatus("checking");
    setBootError(null);
    void apiGet<MeResponse>("/api/admin/me")
      .then((me) => {
        if (cancelled) return;
        setUsername(me.username);
        setBootError(null);
        setReauthRequired(false);
        setStatus("authed");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setUsername(null);
        if (err instanceof ApiError && err.status === 401) {
          setReauthRequired(err.message === "reauth_required");
          setBootError(null);
          setStatus("anonymous");
          // Clear create-post seed on this entry when present; skip if already clean
          // (avoids replace loops / checking flash on anonymous boot with no state).
          if (location.state != null) {
            clearCurrentHistoryState();
          }
          return;
        }
        setBootError(err instanceof ApiError ? err.message : "Failed to check session");
        setStatus("error");
      });
    return () => {
      cancelled = true;
    };
    // Intentionally [retryToken] only: including location.state would re-run the
    // check after clearCurrentHistoryState and flash "checking".
  }, [retryToken]);

  const retry = useCallback(() => {
    setRetryToken((n) => n + 1);
  }, []);

  /** Drop local session and clear the current history entry's state (create-post seed). */
  const dropToAnonymous = useCallback((opts?: { reauth?: boolean }) => {
    setUsername(null);
    setBootError(null);
    setReauthRequired(opts?.reauth === true);
    setStatus("anonymous");
    clearCurrentHistoryState();
  }, [clearCurrentHistoryState]);

  const onUnauthorized = useCallback((opts?: { reauth?: boolean }) => {
    dropToAnonymous(opts);
  }, [dropToAnonymous]);

  const login = useCallback(async (user: string, password: string) => {
    try {
      await apiPost<LoginOk>("/api/admin/login", { username: user, password });
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 401) {
        throw new Error("Wrong username or password");
      }
      if (err instanceof ApiError && err.status === 429) {
        throw new Error("Too many attempts — try again later");
      }
      if (err instanceof Error) throw err;
      throw new Error("Login failed");
    }
    const me = await apiGet<MeResponse>("/api/admin/me");
    setUsername(me.username);
    setBootError(null);
    setReauthRequired(false);
    setStatus("authed");
  }, []);

  const logout = useCallback(async () => {
    // Flip anonymous first so RequireAdmin unmounts the shell immediately.
    dropToAnonymous();
    try {
      await apiPost<LoginOk>("/api/admin/logout", {});
    } catch {
      // Cookie clear is best-effort; local session already dropped.
    }
  }, [dropToAnonymous]);

  const value = useMemo(
    () => ({
      status,
      username,
      bootError,
      reauthRequired,
      login,
      logout,
      retry,
      onUnauthorized,
    }),
    [status, username, bootError, reauthRequired, login, logout, retry, onUnauthorized],
  );

  return (
    <AdminSessionContext.Provider value={value}>{children}</AdminSessionContext.Provider>
  );
}

export function useAdminSession(): AdminSessionValue {
  const ctx = useContext(AdminSessionContext);
  if (!ctx) throw new Error("useAdminSession must be used inside <AdminSessionProvider>");
  return ctx;
}

/** If err is a 401 ApiError, bounce to anonymous and return true. */
export function handleAdminUnauthorized(
  err: unknown,
  onUnauthorized: (opts?: { reauth?: boolean }) => void,
): boolean {
  if (err instanceof ApiError && err.status === 401) {
    onUnauthorized({ reauth: err.message === "reauth_required" });
    return true;
  }
  return false;
}
