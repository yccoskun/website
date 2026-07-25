/** Envelope returned by every backend API endpoint. */
export interface ApiEnvelope<T> {
  data: T | null;
  error: string | null;
}

export interface HealthStatus {
  status: string;
}
