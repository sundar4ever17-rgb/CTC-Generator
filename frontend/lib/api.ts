export class ApiError extends Error { constructor(message: string, public status: number) { super(message); } }
type RequestOptions = { method?: string; body?: unknown; csrf?: string; signal?: AbortSignal; timeoutMs?: number };
async function responseFor(path: string, options: RequestOptions = {}): Promise<Response> {
  const controller = new AbortController();
  const forwardAbort = () => controller.abort(options.signal?.reason);
  if (options.signal?.aborted) forwardAbort();
  options.signal?.addEventListener("abort", forwardAbort, { once: true });
  const timeout = setTimeout(() => controller.abort(), options.timeoutMs ?? 30000);
  try {
    const response = await fetch(`/api${path}`, {
      method: options.method || "GET", credentials: "same-origin", cache: "no-store", signal: controller.signal,
      headers: { ...(options.body !== undefined ? { "Content-Type": "application/json" } : {}), ...(options.csrf ? { "X-CSRF-Token": options.csrf } : {}) },
      ...(options.body !== undefined ? { body: JSON.stringify(options.body) } : {}),
    });
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      if (response.status === 401 && !["/auth/login", "/bootstrap", "/auth/me"].includes(path) && typeof window !== "undefined") window.dispatchEvent(new Event("ctc:session-expired"));
      const fallback = response.status === 429 ? "Too many requests. Wait a moment before trying again." : `Request failed (${response.status}). Please try again.`;
      throw new ApiError(typeof data.error === "string" ? data.error : fallback, response.status);
    }
    return response;
  } catch (error) {
    if (controller.signal.aborted && !options.signal?.aborted) throw new Error("The request timed out. Please try again.");
    if (error instanceof TypeError) throw new Error("Unable to reach the server. Check your connection and try again.");
    throw error;
  } finally {
    clearTimeout(timeout);
    options.signal?.removeEventListener("abort", forwardAbort);
  }
}
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await responseFor(path, options);
  try { return await response.json() as T; }
  catch { throw new Error("The server returned an unexpected response. Please try again."); }
}
export async function downloadPDF(path: string, filename: string): Promise<void> {
  const response = await responseFor(path, { timeoutMs: 120000 });
  if (!response.headers.get("Content-Type")?.includes("application/pdf")) throw new Error("The server did not return a PDF document.");
  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectUrl;
  link.download = filename.replace(/[\\/:*?"<>|\u0000-\u001f]/g, "_");
  document.body.appendChild(link); link.click(); link.remove();
  setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
}
export function money(value?: string | null): string {
  if (value == null || value === "") return "—";
  const negative = value.startsWith("-");
  const [whole, fraction = ""] = value.replace(/^-/, "").split(".");
  try { return `${negative ? "−" : ""}₹${new Intl.NumberFormat("en-IN").format(BigInt(whole || "0"))}${fraction && !/^0+$/.test(fraction) ? `.${fraction.padEnd(2, "0")}` : ""}`; } catch { return value; }
}
export function dateTime(value: string): string { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("en-IN", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Kolkata", timeZoneName: "short" }).format(date); }
export function errorMessage(error: unknown): string { return error instanceof Error ? error.message : "Something went wrong. Please try again."; }
