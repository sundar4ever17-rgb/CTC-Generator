"use client";
import { useEffect, useRef, useState } from "react";
import { ArrowRight, Calculator, FileClock, Settings2, ShieldCheck, Users, LogOut, Layers3, PanelLeftClose, X, CircleHelp, ChevronRight, KeyRound } from "lucide-react";
import { request, ApiError, errorMessage } from "@/lib/api";
import { Bootstrap, CandidateInput, Configuration, User, emptyInput, demoInput, roleNames } from "@/lib/types";
import { Badge, Notice, Spinner, Modal } from "@/components/ui";
import Generator from "@/components/generator";
import Records from "@/components/records";
import ConfigurationEditor from "@/components/configuration";
import PasswordChange from "@/components/password-change";
import SecuritySettings from "@/components/security-settings";
import { AuditView, UsersView } from "@/components/admin";

type Tab = "generate" | "records" | "configuration" | "audit" | "users" | "settings";
type Auth = { user: User; csrfToken: string };
const nav = [{ key: "generate", title: "Generate CTC", icon: Calculator }, { key: "records", title: "CTC records", icon: FileClock }, { key: "configuration", title: "Configuration", icon: Settings2 }, { key: "audit", title: "Audit history", icon: ShieldCheck }, { key: "users", title: "User management", icon: Users }, { key: "settings", title: "Security settings", icon: KeyRound }] as const;
export default function Home() {
  const [boot, setBoot] = useState<Bootstrap | null>(null);
  const [passwordRequired, setPasswordRequired] = useState<Auth | null>(null);
  const [loading, setLoading] = useState(true);
  const [loginError, setLoginError] = useState("");
  const [tab, setTab] = useState<Tab>("generate");
  const [toast, setToast] = useState<{ text: string; error?: boolean } | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [mobileNav, setMobileNav] = useState(false);
  const [input, setInput] = useState<CandidateInput>(() => structuredClone(emptyInput));
  const [configurationId, setConfigurationId] = useState("");
  const [caseId, setCaseId] = useState<string | undefined>();
  const [expectedRevisionId, setExpectedRevisionId] = useState<string | undefined>();
  const activeUser = useRef("");
  const authEpoch = useRef(0);
  function clearSession(message = "") {
    authEpoch.current += 1; activeUser.current = "";
    setBoot(null); setPasswordRequired(null); setCaseId(undefined); setExpectedRevisionId(undefined);
    setInput(structuredClone(emptyInput)); setConfigurationId(""); setTab("generate");
    setShowHelp(false); setShowPassword(false); setMobileNav(false); setToast(null); setLoginError(message);
  }
  async function load() {
    const epoch = authEpoch.current;
    try {
      const auth = await request<Auth>("/auth/me");
      if (epoch !== authEpoch.current) return;
      if (auth.user.mustChangePassword) { setPasswordRequired(auth); setBoot(null); return; }
      const b = await request<Bootstrap>("/bootstrap");
      if (epoch !== authEpoch.current) return;
      if (activeUser.current !== b.user.id) {
        const first = (b.demo ? b.configurations.find(c => c.source === "urd-demo") : undefined) || b.configurations.find(c => c.status === "published" && c.source === "client") || b.configurations[0];
        setInput({ ...structuredClone(b.demo && first?.source === "urd-demo" ? demoInput : emptyInput), gradeId: first?.data.grades[0]?.id || "", locationId: first?.data.locations[0]?.id || "" });
        setCaseId(undefined); setExpectedRevisionId(undefined); setConfigurationId(first?.id || "");
        setTab(b.user.role === "auditor" ? "records" : b.user.role === "admin" ? "users" : b.user.role === "cb" && !first ? "configuration" : "generate");
      }
      activeUser.current = b.user.id; setPasswordRequired(null); setBoot(b); setLoginError("");
    } catch (e) {
      if (epoch === authEpoch.current) clearSession(e instanceof ApiError && e.status === 401 ? "" : errorMessage(e));
    } finally { setLoading(false); }
  }
  useEffect(() => { load(); }, []);
  useEffect(() => {
    const expired = () => clearSession("Your session has expired. Please sign in again.");
    window.addEventListener("ctc:session-expired", expired);
    return () => window.removeEventListener("ctc:session-expired", expired);
  }, []);
  useEffect(() => { if (toast) { const id = setTimeout(() => setToast(null), 6000); return () => clearTimeout(id); } }, [toast]);
  const notify = (text: string, error = false) => setToast({ text, error });
  async function logout() {
    try { await request("/auth/logout", { method: "POST", csrf: boot?.csrfToken || passwordRequired?.csrfToken }); clearSession(); }
    catch (e) { notify(errorMessage(e), true); }
  }
  const passwordChanged = () => clearSession("Your password was updated. Please sign in with your new password.");
  if (loading) return <main className="splash"><div className="brand-mark"><Layers3 size={30} /></div><h1>CTC Studio</h1><Spinner label="Opening your compensation workspace" /></main>;
  if (passwordRequired) return <main className="splash"><section className="card password-required"><div className="modal-heading"><h1>Set your personal password</h1></div><PasswordChange csrf={passwordRequired.csrfToken} required onSuccess={passwordChanged} /><button className="button button-ghost" onClick={logout}>Sign out</button></section></main>;
  if (!boot) return <Login onSuccess={load} error={loginError} />;
  const sessionEpoch = authEpoch.current;
  const currentSession = () => sessionEpoch === authEpoch.current && activeUser.current === boot.user.id;
  const title = nav.find(n => n.key === tab)?.title || "Workspace";
  return <div className="app-shell">
    {mobileNav && <div className="nav-scrim" onClick={() => setMobileNav(false)} />}
    <aside className={`sidebar ${mobileNav ? "sidebar-open" : ""}`}>
      <a href="#" className="brand" onClick={e => { e.preventDefault(); setTab(boot.user.role === "auditor" ? "records" : "generate"); }}><span className="brand-mark"><Layers3 size={24} /></span><span>CTC<span className="brand-light"> Studio</span><small>COMPENSATION WORKSPACE</small></span></a>
      <div className="workspace-label">WORKSPACE</div>
      <nav aria-label="Main navigation">{nav.filter(n => (!["users", "settings"].includes(n.key) || boot.user.role === "admin") && (n.key !== "generate" || boot.user.role !== "auditor")).map(n => <button key={n.key} onClick={() => { setTab(n.key); setMobileNav(false); }} className={`nav-item ${tab === n.key ? "active" : ""}`} aria-current={tab === n.key ? "page" : undefined}><n.icon size={19} /><span>{n.title}</span>{tab === n.key && <span className="nav-active-dot" />}</button>)}</nav>
      <div className="sidebar-bottom"><div className="sidebar-note"><span className="live-dot" /><strong>{boot.demo ? "Demo environment" : "Compensation workspace"}</strong><p>Versioned rules and traceable calculations.</p></div><button className="nav-item" onClick={() => setShowHelp(true)}><CircleHelp size={18} />Rules & clarifications</button><button className="nav-item" onClick={() => setShowPassword(true)}><KeyRound size={18} />Change password</button><div className="profile"><div className="avatar">{boot.user.name.slice(0, 1).toUpperCase()}</div><div><strong>{boot.user.name}</strong><small>{roleNames[boot.user.role]}</small></div><button className="icon-button" title="Sign out" aria-label="Sign out" onClick={logout}><LogOut size={17} /></button></div></div>
    </aside>
    <div className="workspace"><header className="topbar"><div className="breadcrumb"><button className="icon-button mobile-menu" aria-label="Open navigation" onClick={() => setMobileNav(true)}><PanelLeftClose size={20} /></button><span>Workspace</span><ChevronRight size={14} /><strong>{title}</strong></div><div className="topbar-actions">{boot.demo && <Badge tone="amber">DEMO</Badge>}<span className="topbar-divider" /><span className="topbar-context">India · INR</span></div></header>
      <main className="main-content" key={boot.user.id}>
        {tab === "generate" && boot.user.role !== "auditor" && <Generator boot={boot} input={input} setInput={next => { if (currentSession()) setInput(next); }} configurationId={configurationId} setConfigurationId={id => { if (currentSession()) setConfigurationId(id); }} caseId={caseId} setCaseId={id => { if (currentSession()) setCaseId(id); }} expectedRevisionId={expectedRevisionId} setExpectedRevisionId={id => { if (currentSession()) setExpectedRevisionId(id); }} notify={notify} />}
        {tab === "records" && <Records notify={notify} user={boot.user} csrf={boot.csrfToken} onContinue={async (revision, id, latestRevisionId) => { try { if (!boot.configurations.some(c => c.id === revision.configurationId)) { const configuration = await request<Configuration>(`/configurations/${revision.configurationId}`); if (!currentSession()) return; setBoot(current => current ? { ...current, configurations: [...current.configurations, configuration] } : current); } if (!currentSession()) return; setInput(revision.input); setConfigurationId(revision.configurationId); setCaseId(id); setExpectedRevisionId(latestRevisionId); setTab("generate"); } catch (e) { notify(errorMessage(e), true); } }} />}
        {tab === "configuration" && <ConfigurationEditor boot={boot} notify={notify} onCreated={async (config: Configuration) => { if (!currentSession()) return; await load(); if (!currentSession()) return; setConfigurationId(config.id); notify("Configuration version saved. Earlier versions remain unchanged."); }} />}
        {tab === "audit" && <AuditView notify={notify} />}
        {tab === "settings" && boot.user.role === "admin" && <SecuritySettings csrf={boot.csrfToken} notify={notify} />}
        {tab === "users" && boot.user.role === "admin" && <UsersView user={boot.user} csrf={boot.csrfToken} demo={boot.demo} notify={notify} />}
      </main>
      <footer className="workspace-footer"><span>CTC Studio <span className="footer-dot">·</span> Designed for clear compensation decisions</span><span>All calculations follow a versioned rule set</span></footer>
    </div>
    {toast && <div className={`toast ${toast.error ? "toast-error" : ""}`} role={toast.error ? "alert" : "status"}><span>{toast.text}</span><button onClick={() => setToast(null)} className="icon-button" aria-label="Dismiss notification"><X size={16} /></button></div>}
    {showPassword && <Modal title="Change your password" onClose={() => setShowPassword(false)}><PasswordChange csrf={boot.csrfToken} onSuccess={passwordChanged} /></Modal>}
    {showHelp && <Modal title="Rules & clarifications" onClose={() => setShowHelp(false)} wide><div className="modal-body"><Notice tone="blue">The URD defines the calculation sequence. Missing policies are shown explicitly and must be confirmed before final offers are generated.</Notice><div className="clarification-list">{boot.clarifications.map(c => <article key={c.id}><Badge>{c.id}</Badge><div><h3>{c.title}</h3><p>{c.detail}</p></div></article>)}</div></div></Modal>}
  </div>;
}
function Login({ onSuccess, error }: { onSuccess: () => Promise<void>; error: string }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState(error);
  useEffect(() => setMessage(error), [error]);
  async function submit(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setMessage("");
    try { await request<Auth>("/auth/login", { method: "POST", body: { email: email.trim(), password } }); setPassword(""); await onSuccess(); }
    catch (err) { setMessage(errorMessage(err)); }
    finally { setBusy(false); }
  }
  return <main className="login-page"><section className="login-story"><a className="brand" href="#"><span className="brand-mark"><Layers3 size={26} /></span><span>CTC<span className="brand-light"> Studio</span></span></a><div className="login-story-content"><span className="eyebrow light">COMPENSATION, WITH CLARITY</span><h1>Every component.<br />A clear picture.</h1><p>Build balanced salary structures, understand every calculation, and prepare consistent offers in one considered workspace.</p><div className="login-benefits"><span><ShieldCheck size={19} /> Traceable rules and revisions</span><span><Calculator size={19} /> Accurate, sequential calculations</span><span><FileClock size={19} /> Consistent compensation documents</span></div></div><div className="login-story-footer"><span className="live-dot" /> Your compensation workspace</div><div className="story-orbit orbit-one" /><div className="story-orbit orbit-two" /></section><section className="login-form-panel"><div className="login-form-container"><h2>Welcome to CTC Studio</h2><p className="subtext">Sign in to your compensation workspace.</p><form onSubmit={submit}><label className="field"><span className="field-label">Email address</span><input type="email" autoComplete="username" required maxLength={254} value={email} onChange={e => setEmail(e.target.value)} disabled={busy} /></label><label className="field"><span className="field-label">Password</span><input type="password" autoComplete="current-password" required value={password} onChange={e => setPassword(e.target.value)} disabled={busy} /></label>{message && <Notice tone="red">{message}</Notice>}<button className="button button-primary login-submit" disabled={busy}>{busy ? <Spinner label="Signing in" /> : <>Sign in <ArrowRight size={18} /></>}</button></form><p className="login-policy">Contact your application administrator if you need an account or a password reset.</p></div></section></main>;
}
