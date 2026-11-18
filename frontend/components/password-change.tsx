"use client";
import { useState } from "react";
import { request, errorMessage } from "@/lib/api";
import { Field, Notice, Spinner } from "./ui";

export default function PasswordChange({ csrf, required = false, onSuccess }: { csrf: string; required?: boolean; onSuccess: () => void }) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (newPassword !== confirmation) { setError("The new passwords do not match."); return; }
    const length = new TextEncoder().encode(newPassword).length;
    if (length < 15 || length > 72) { setError("Use a password between 15 and 72 bytes (15–72 basic Latin characters)."); return; }
    setBusy(true); setError("");
    try {
      await request("/auth/password", { method: "POST", csrf, body: { currentPassword, newPassword } });
      setCurrentPassword(""); setNewPassword(""); setConfirmation(""); onSuccess();
    } catch (e) { setError(errorMessage(e)); }
    finally { setBusy(false); }
  }
  return <form className="modal-body" onSubmit={submit}>
    <Notice tone="blue">{required ? "Choose a personal password before opening your workspace." : "Changing your password signs out all sessions. Sign in again with your new password."}</Notice>
    <Field label="Current password"><input type="password" autoComplete="current-password" required value={currentPassword} onChange={e => setCurrentPassword(e.target.value)} disabled={busy} /></Field>
    <Field label="New password" hint="Use a unique passphrase of at least 15 characters."><input type="password" autoComplete="new-password" required minLength={15} maxLength={72} value={newPassword} onChange={e => setNewPassword(e.target.value)} disabled={busy} /></Field>
    <Field label="Confirm new password"><input type="password" autoComplete="new-password" required minLength={15} maxLength={72} value={confirmation} onChange={e => setConfirmation(e.target.value)} disabled={busy} /></Field>
    {error && <Notice tone="red">{error}</Notice>}
    <div className="modal-actions"><button className="button button-primary" disabled={busy}>{busy ? <Spinner label="Updating password" /> : "Update password"}</button></div>
  </form>;
}
