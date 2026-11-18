"use client";
import { useEffect, useState } from "react";
import { errorMessage, request } from "./api";

export function usePagedResource<T>(path: string, search: string, pageSize = 50) {
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [page, setPage] = useState(0);
  const [version, setVersion] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError("");
    const timer = setTimeout(async () => {
      try {
        const query = new URLSearchParams({ q: search.trim(), limit: String(pageSize), offset: String(page * pageSize) });
        const result = await request<T[]>(`${path}?${query}`, { signal: controller.signal });
        if (!controller.signal.aborted) setItems(result || []);
      } catch (e) {
        if (!controller.signal.aborted) { setItems([]); setError(errorMessage(e)); }
      } finally { if (!controller.signal.aborted) setLoading(false); }
    }, search ? 300 : 0);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [path, search, page, pageSize, version]);
  return { items, loading, error, page, pageSize, setPage, reload: () => setVersion(v => v + 1) };
}
