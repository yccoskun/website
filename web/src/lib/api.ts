import type { ApiEnvelope } from "@/types/api";

/** Typed API failure with HTTP status for callers that branch on 404 etc. */
export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function parseEnvelope<T>(path: string, method: string, res: Response): Promise<T> {
  const contentType = res.headers.get("content-type") ?? "";
  if (!contentType.includes("application/json")) {
    throw new ApiError(
      `${method} ${path} failed with status ${res.status} (non-JSON response)`,
      res.status,
    );
  }
  const body = (await res.json()) as ApiEnvelope<T>;
  if (body.error !== null) throw new ApiError(body.error, res.status);
  if (body.data === null) throw new ApiError(`empty response from ${path}`, res.status);
  return body.data;
}

/** Fetches a backend endpoint and unwraps the {data, error} envelope. */
export async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(path, { credentials: "include" });
  return parseEnvelope<T>(path, "GET", res);
}

/** POSTs JSON to a backend endpoint with credentials. */
export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return parseEnvelope<T>(path, "POST", res);
}

/** PUTs JSON to a backend endpoint with credentials. */
export async function apiPut<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "PUT",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return parseEnvelope<T>(path, "PUT", res);
}

/** DELETEs a backend endpoint with credentials. */
export async function apiDelete<T>(path: string): Promise<T> {
  const res = await fetch(path, {
    method: "DELETE",
    credentials: "include",
  });
  return parseEnvelope<T>(path, "DELETE", res);
}
