export class ApiError extends Error { constructor(message: string, public status: number) { super(message); } }
export async function request<T>(path: string, options: { method?: string; body?: unknown; csrf?: string; signal?: AbortSignal } = {}): Promise<T> {
  const response = await fetch(`/api${path}`, { method: options.method || "GET", credentials: "same-origin", cache: "no-store", signal: options.signal, headers: { ...(options.body !== undefined ? { "Content-Type": "application/json" } : {}), ...(options.csrf ? { "X-CSRF-Token": options.csrf } : {}) }, ...(options.body !== undefined ? { body: JSON.stringify(options.body) } : {}) });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) { if (response.status === 401 && path !== "/auth/login" && path !== "/bootstrap" && typeof window !== "undefined") window.dispatchEvent(new Event("ctc:session-expired")); throw new ApiError(data.error || `Request failed (${response.status}). Please try again.`, response.status); }
  return data as T;
}
export function money(value?: string | null): string {
  if (value == null || value === "") return "—";
  const negative = value.startsWith("-");
  const [whole, fraction = ""] = value.replace(/^-/, "").split(".");
  try { return `${negative ? "−" : ""}₹${new Intl.NumberFormat("en-IN").format(BigInt(whole || "0"))}${fraction && !/^0+$/.test(fraction) ? `.${fraction.padEnd(2, "0")}` : ""}`; } catch { return value; }
}
export function dateTime(value: string): string { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("en-IN", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(date); }
export function errorMessage(error: unknown): string { return error instanceof Error ? error.message : "Something went wrong. Please try again."; }
