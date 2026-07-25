import { useEffect, useState } from "react";

import { ApiError, apiGet } from "@/lib/api";

export interface UseApiResult<T> {
  data: T | null;
  loading: boolean;
  error: ApiError | null;
}

function toApiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err;
  if (err instanceof Error) return new ApiError(err.message, 0);
  return new ApiError("request failed", 0);
}

/** Fetches JSON via apiGet. Pass null path to skip. Ignores stale responses. */
export function useApi<T>(path: string | null): UseApiResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(path !== null);
  const [error, setError] = useState<ApiError | null>(null);

  useEffect(() => {
    if (path === null) {
      setData(null);
      setLoading(false);
      setError(null);
      return;
    }

    let cancelled = false;
    setData(null);
    setLoading(true);
    setError(null);

    void apiGet<T>(path)
      .then((result) => {
        if (cancelled) return;
        setData(result);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setData(null);
        setError(toApiError(err));
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [path]);

  return { data, loading, error };
}
