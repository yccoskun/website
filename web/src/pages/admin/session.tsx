import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { ReactNode } from "react";

import { ApiError, apiGet, apiPost } from "@/lib/api";
import type { LoginOk, MeResponse } from "@/types/admin";

export type AdminSessionStatus = "checking" | "anonymous" | "authed" | "error";

interface AdminSessionValue {
  status: AdminSessionStatus;
  username: string | null;
  bootError: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  retry: () => void;
  onUnauthorized: () => void;
}

const AdminSessionContext = createContext<AdminSessionValue | null>(null);

export function AdminSessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AdminSessionStatus>("checking");
  const [username, setUsername] = useState<string | null>(null);
  const [bootError, setBootError] = useState<string | null>(null);
  const [retryToken, setRetryToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setStatus("checking");
    setBootError(null);
    void apiGet<MeResponse>("/api/admin/me")
      .then((me) => {
        if (cancelled) return;
        setUsername(me.username);
        setBootError(null);
        setStatus("authed");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setUsername(null);
        if (err instanceof ApiError && err.status === 401) {
          setBootError(null);
          setStatus("anonymous");
          return;
        }
        setBootError(err instanceof ApiError ? err.message : "Failed to check session");
        setStatus("error");
      });
    return () => {
      cancelled = true;
    };
  }, [retryToken]);

  const retry = useCallback(() => {
    setRetryToken((n) => n + 1);
  }, []);

  const onUnauthorized = useCallback(() => {
    setUsername(null);
    setBootError(null);
    setStatus("anonymous");
  }, []);

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
    setStatus("authed");
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiPost<LoginOk>("/api/admin/logout", {});
    } catch {
      // Cookie clear is best-effort; always drop local session.
    }
    setUsername(null);
    setBootError(null);
    setStatus("anonymous");
  }, []);

  const value = useMemo(
    () => ({ status, username, bootError, login, logout, retry, onUnauthorized }),
    [status, username, bootError, login, logout, retry, onUnauthorized],
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
  onUnauthorized: () => void,
): boolean {
  if (err instanceof ApiError && err.status === 401) {
    onUnauthorized();
    return true;
  }
  return false;
}
