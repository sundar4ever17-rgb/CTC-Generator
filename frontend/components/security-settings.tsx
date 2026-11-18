"use client";
import { useEffect, useState } from "react";
import { Save, Settings2 } from "lucide-react";
import { request, errorMessage } from "@/lib/api";
import { Field, Notice, Spinner } from "./ui";

type Settings = { sessionIdleMinutes: number; sessionMaxHours: number };
export default function SecuritySettings({ csrf, notify }: { csrf: string; notify: (message: string, error?: boolean) => void }) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function load() {
    setError("");
    try { setSettings(await request<Settings>("/settings")); }
    catch (e) { setError(errorMessage(e)); }
  }
  useEffect(() => { load(); }, []);
  async function save(event: React.FormEvent) {
    event.preventDefault();
    if (!settings) return;
    setBusy(true); setError("");
    try {
      await request("/settings", { method: "PATCH", csrf, body: { ...settings, currentPassword: password } });
      setPassword(""); await load(); notify("Session security settings updated.");
    } catch (e) { setError(errorMessage(e)); }
    finally { setBusy(false); }
  }
  return <><div className="page-heading"><div><span className="eyebrow">ADMINISTRATION</span><h1>Security settings</h1><p>Manage session limits for the application.</p></div><Settings2 size={24} /></div>
    {error && <Notice tone="red">{error}</Notice>}
    {!settings ? <div className="card config-section">{error ? <button className="button button-secondary" onClick={load}>Retry</button> : <Spinner label="Loading security settings" />}</div> : <form className="card config-section" onSubmit={save}>
      <Notice tone="blue">Changes are audited. Enter your current administrator password to save these security controls.</Notice>
      <div className="field-grid"><Field label="Idle session timeout (minutes)" hint="Sign out inactive sessions after 5 to 60 minutes."><input type="number" min={5} max={60} step={1} required value={settings.sessionIdleMinutes} disabled={busy} onChange={e => setSettings(s => s && ({ ...s, sessionIdleMinutes: Number(e.target.value) }))} /></Field>
        <Field label="Maximum session duration (hours)" hint="Require sign-in again after 1 to 24 hours, even while active."><input type="number" min={1} max={24} step={1} required value={settings.sessionMaxHours} disabled={busy} onChange={e => setSettings(s => s && ({ ...s, sessionMaxHours: Number(e.target.value) }))} /></Field></div>
      <Field label="Current administrator password"><input type="password" autoComplete="current-password" required value={password} disabled={busy} onChange={e => setPassword(e.target.value)} /></Field>
      <div className="modal-actions"><button className="button button-primary" disabled={busy}>{busy ? <Spinner label="Saving settings" /> : <><Save size={16} />Save security settings</>}</button></div>
    </form>}
  </>;
}
