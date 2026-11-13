"use client";
import { useEffect, useState } from "react";
import { ArrowRight, Calculator, FileClock, Settings2, ShieldCheck, Users, LogOut, Layers3, PanelLeftClose, X, CircleHelp, ChevronRight, Sparkles } from "lucide-react";
import { request, ApiError, errorMessage } from "@/lib/api";
import { Bootstrap, CandidateInput, Configuration, User, emptyInput, roleNames } from "@/lib/types";
import { Badge, Notice, Spinner, Modal } from "@/components/ui";
import Generator from "@/components/generator";
import Records from "@/components/records";
import ConfigurationEditor from "@/components/configuration";
import { AuditView, UsersView } from "@/components/admin";

type Tab = "generate" | "records" | "configuration" | "audit" | "users";
const nav = [{ key: "generate", title: "Generate CTC", icon: Calculator }, { key: "records", title: "CTC records", icon: FileClock }, { key: "configuration", title: "Configuration", icon: Settings2 }, { key: "audit", title: "Audit history", icon: ShieldCheck }, { key: "users", title: "User management", icon: Users }] as const;
export default function Home() {
  const [boot, setBoot] = useState<Bootstrap | null>(null);
  const [loading, setLoading] = useState(true);
  const [loginError, setLoginError] = useState("");
  const [tab, setTab] = useState<Tab>("generate");
  const [toast, setToast] = useState<{ text: string; error?: boolean } | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);
  const [input, setInput] = useState<CandidateInput>(emptyInput);
  const [configurationId, setConfigurationId] = useState("");
  const [caseId, setCaseId] = useState<string | undefined>();
  async function load() {
    try { const b = await request<Bootstrap>("/bootstrap"); setBoot(b); if (b.user.role === "auditor") setTab("records"); if (!configurationId && b.configurations?.length) { setConfigurationId(b.configurations[0].id); setInput(i => ({ ...i, gradeId: b.configurations[0].data.grades[0]?.id || "", locationId: b.configurations[0].data.locations[0]?.id || "" })); } setLoginError(""); }
    catch (e) { if (!(e instanceof ApiError && e.status === 401)) setLoginError(errorMessage(e)); setBoot(null); }
    finally { setLoading(false); }
  }
  useEffect(() => { load(); }, []); // Initial session restore.
  useEffect(() => { const expired = () => { setBoot(null); setLoginError("Your session has expired. Please sign in again."); }; window.addEventListener("ctc:session-expired", expired); return () => window.removeEventListener("ctc:session-expired", expired); }, []);
  useEffect(() => { if (toast) { const id = setTimeout(() => setToast(null), 6000); return () => clearTimeout(id); } }, [toast]);
  const notify = (text: string, error = false) => setToast({ text, error });
  async function logout() { try { await request("/auth/logout", { method: "POST", csrf: boot?.csrfToken }); setBoot(null); setCaseId(undefined); setInput(emptyInput); setConfigurationId(""); } catch (e) { notify(errorMessage(e), true); } }
  if (loading) return <main className="splash"><div className="brand-mark"><Layers3 size={30} /></div><h1>CTC Studio</h1><Spinner label="Opening your compensation workspace" /></main>;
  if (!boot) return <Login onSuccess={load} error={loginError} />;
  const title = nav.find(n => n.key === tab)?.title || "Workspace";
  return <div className="app-shell">
    {mobileNav && <div className="nav-scrim" onClick={() => setMobileNav(false)} />}
    <aside className={`sidebar ${mobileNav ? "sidebar-open" : ""}`}>
      <a href="#" className="brand" onClick={e => { e.preventDefault(); setTab(boot.user.role === "auditor" ? "records" : "generate"); }}><span className="brand-mark"><Layers3 size={24} /></span><span>CTC<span className="brand-light"> Studio</span><small>COMPENSATION WORKSPACE</small></span></a>
      <div className="workspace-label">WORKSPACE</div>
      <nav aria-label="Main navigation">{nav.filter(n => (n.key !== "users" || boot.user.role === "admin") && (n.key !== "generate" || boot.user.role !== "auditor")).map(n => <button key={n.key} onClick={() => { setTab(n.key); setMobileNav(false); }} className={`nav-item ${tab === n.key ? "active" : ""}`}><n.icon size={19} /><span>{n.title}</span>{tab === n.key && <span className="nav-active-dot" />}</button>)}</nav>
      <div className="sidebar-bottom"><div className="sidebar-note"><span className="live-dot" /><strong>Demo environment</strong><p>URD-based calculations.<br />Policy decisions stay visible.</p></div><button className="nav-item" onClick={() => setShowHelp(true)}><CircleHelp size={18} />Rules & clarifications</button><div className="profile"><div className="avatar">{boot.user.name.slice(0, 1).toUpperCase()}</div><div><strong>{boot.user.name}</strong><small>{roleNames[boot.user.role]}</small></div><button className="icon-button" title="Sign out" aria-label="Sign out" onClick={logout}><LogOut size={17} /></button></div></div>
    </aside>
    <div className="workspace"><header className="topbar"><div className="breadcrumb"><button className="icon-button mobile-menu" aria-label="Open navigation" onClick={() => setMobileNav(true)}><PanelLeftClose size={20} /></button><span>Workspace</span><ChevronRight size={14} /><strong>{title}</strong></div><div className="topbar-actions"><Badge tone="amber">DEMO</Badge><span className="topbar-divider" /><span className="topbar-context">India · INR</span></div></header>
      <main className="main-content">
        {tab === "generate" && <Generator boot={boot} input={input} setInput={setInput} configurationId={configurationId} setConfigurationId={setConfigurationId} caseId={caseId} setCaseId={setCaseId} notify={notify} />}
        {tab === "records" && <Records notify={notify} user={boot.user} onContinue={(revision, id) => { setInput(revision.input); setConfigurationId(revision.configurationId); setCaseId(id); setTab("generate"); }} />}
        {tab === "configuration" && <ConfigurationEditor boot={boot} notify={notify} onCreated={async (config: Configuration) => { await load(); setConfigurationId(config.id); notify("Configuration version saved. Earlier versions remain unchanged."); }} />}
        {tab === "audit" && <AuditView notify={notify} />}
        {tab === "users" && boot.user.role === "admin" && <UsersView user={boot.user} csrf={boot.csrfToken} notify={notify} />}
      </main>
      <footer className="workspace-footer"><span>CTC Studio <span className="footer-dot">·</span> Designed for clear compensation decisions</span><span>All calculations follow a versioned rule set</span></footer>
    </div>
    {toast && <div className={`toast ${toast.error ? "toast-error" : ""}`} role={toast.error ? "alert" : "status"}><span>{toast.text}</span><button onClick={() => setToast(null)} className="icon-button" aria-label="Dismiss notification"><X size={16} /></button></div>}
    {showHelp && <Modal title="Rules & clarifications" onClose={() => setShowHelp(false)} wide><div className="modal-body"><Notice tone="blue">The URD defines the calculation sequence. Missing policies are shown explicitly and must be confirmed before final offers are generated.</Notice><div className="clarification-list">{boot.clarifications.map(c => <article key={c.id}><Badge>{c.id}</Badge><div><h3>{c.title}</h3><p>{c.detail}</p></div></article>)}</div></div></Modal>}
  </div>;
}
function Login({ onSuccess, error }: { onSuccess: () => Promise<void>; error: string }) {
  const [email, setEmail] = useState("cb@demo.local"); const [password, setPassword] = useState("DemoCTC!2026"); const [busy, setBusy] = useState(false); const [message, setMessage] = useState(error);
  useEffect(() => setMessage(error), [error]);
  async function submit(e: React.FormEvent) { e.preventDefault(); setBusy(true); setMessage(""); try { await request<{ user: User; csrfToken: string }>("/auth/login", { method: "POST", body: { email, password } }); await onSuccess(); } catch (err) { setMessage(errorMessage(err)); } finally { setBusy(false); } }
  return <main className="login-page"><section className="login-story"><a className="brand" href="#"><span className="brand-mark"><Layers3 size={26} /></span><span>CTC<span className="brand-light"> Studio</span></span></a><div className="login-story-content"><span className="eyebrow light">COMPENSATION, WITH CLARITY</span><h1>Every component.<br />A clear picture.</h1><p>Build balanced salary structures, understand every calculation, and prepare consistent offers in one considered workspace.</p><div className="login-benefits"><span><ShieldCheck size={19} /> Traceable rules and revisions</span><span><Calculator size={19} /> Accurate, sequential calculations</span><span><FileClock size={19} /> Consistent compensation documents</span></div></div><div className="login-story-footer"><span className="live-dot" /> A demo built around your URD</div><div className="story-orbit orbit-one" /><div className="story-orbit orbit-two" /></section><section className="login-form-panel"><div className="login-form-container"><Badge tone="amber">DEMO WORKSPACE</Badge><h2>Welcome to CTC Studio</h2><p className="subtext">Sign in to your compensation workspace.</p><form onSubmit={submit}><label className="field"><span className="field-label">Email address</span><input type="email" autoComplete="username" required value={email} onChange={e => setEmail(e.target.value)} /></label><label className="field"><span className="field-label">Password</span><input type="password" autoComplete="current-password" required value={password} onChange={e => setPassword(e.target.value)} /></label>{message && <Notice tone="red">{message}</Notice>}<button className="button button-primary login-submit" disabled={busy}>{busy ? <Spinner label="Signing in" /> : <>Sign in <ArrowRight size={18} /></>}</button></form><div className="demo-access"><div className="demo-access-title"><Sparkles size={16} /><strong>Explore the demo by role</strong></div><div className="role-choices">{[{ id: "cb", name: "C&B" }, { id: "hr", name: "HR user" }, { id: "admin", name: "Admin" }, { id: "auditor", name: "Auditor" }].map(r => <button key={r.id} type="button" className={email === `${r.id}@demo.local` ? "selected" : ""} onClick={() => { setEmail(`${r.id}@demo.local`); setPassword("DemoCTC!2026"); }}>{r.name}</button>)}</div><p>Local demo accounts only. Password: <code>DemoCTC!2026</code></p></div><p className="login-policy">Illustrative data is clearly marked. Unconfirmed policy rules will never be silently treated as zero.</p></div></section></main>;
}

