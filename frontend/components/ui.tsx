"use client";
import { ReactNode, useEffect, useRef } from "react";
import { LoaderCircle, X, AlertTriangle, CheckCircle2, Info } from "lucide-react";
export function Spinner({ label = "Loading" }: { label?: string }) { return <span className="loading-inline"><LoaderCircle size={17} className="spin" />{label}</span>; }
export function Field({ label, hint, children, className = "" }: { label: string; hint?: string; children: ReactNode; className?: string }) { return <label className={`field ${className}`}><span className="field-label">{label}</span>{children}{hint && <span className="field-hint">{hint}</span>}</label>; }
export function Badge({ children, tone = "neutral" }: { children: ReactNode; tone?: "neutral" | "green" | "amber" | "red" | "blue" }) { return <span className={`badge badge-${tone}`}>{children}</span>; }
export function Notice({ children, tone = "amber" }: { children: ReactNode; tone?: "amber" | "red" | "green" | "blue" }) { const Icon = tone === "green" ? CheckCircle2 : tone === "blue" ? Info : AlertTriangle; return <div className={`notice notice-${tone}`} role={tone === "red" ? "alert" : "status"}><Icon size={18} aria-hidden="true" /><div>{children}</div></div>; }
export function Empty({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) { return <div className="empty-state"><div className="empty-icon">{icon}</div><h3>{title}</h3><p>{children}</p></div>; }
export function Pagination({ page, pageSize, count, loading, onChange }: { page: number; pageSize: number; count: number; loading: boolean; onChange: (page: number) => void }) {
  return <div className="table-toolbar pagination" aria-label="Pagination"><span className="small-text">Page {page + 1} · {count} record{count === 1 ? "" : "s"} shown</span><div className="heading-actions"><button className="button button-secondary" disabled={loading || page === 0} onClick={() => onChange(page - 1)}>Previous page</button><button className="button button-secondary" disabled={loading || count < pageSize} onClick={() => onChange(page + 1)}>Next page</button></div></div>;
}
export function Modal({ title, children, onClose, wide = false }: { title: string; children: ReactNode; onClose: () => void; wide?: boolean }) {
  const panel = useRef<HTMLElement>(null);
  const close = useRef(onClose); close.current = onClose;
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const fields = panel.current?.querySelector<HTMLElement>("input:not([disabled]), select:not([disabled]), textarea:not([disabled])");
    (fields || panel.current?.querySelector<HTMLElement>("button:not([disabled])"))?.focus();
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); close.current(); return; }
      if (event.key !== "Tab" || !panel.current) return;
      const focusable = Array.from(panel.current.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), summary, [tabindex="0"]')).filter(el => el.getClientRects().length > 0);
      if (!focusable.length) { event.preventDefault(); panel.current.focus(); return; }
      const first = focusable[0], last = focusable[focusable.length - 1];
      if (event.shiftKey && (document.activeElement === first || !panel.current.contains(document.activeElement))) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", keyboard);
    return () => { document.removeEventListener("keydown", keyboard); document.body.style.overflow = previousOverflow; previous?.focus(); };
  }, []);
  return <div className="modal-backdrop" onClick={onClose}><section ref={panel} tabIndex={-1} className={`modal ${wide ? "modal-wide" : ""}`} role="dialog" aria-modal="true" aria-label={title} onClick={e => e.stopPropagation()}><div className="modal-heading"><h2>{title}</h2><button className="icon-button" aria-label="Close dialog" onClick={onClose}><X size={20} /></button></div>{children}</section></div>; }
export function Toggle({ checked, onChange, label, disabled = false }: { checked: boolean; onChange: (v: boolean) => void; label: string; disabled?: boolean }) { return <label className="toggle-label"><input type="checkbox" checked={checked} onChange={e => onChange(e.target.checked)} disabled={disabled} /><span className="toggle-track" aria-hidden="true" /><span>{label}</span></label>; }
