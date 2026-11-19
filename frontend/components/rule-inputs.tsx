"use client";
import type { RuleInput, CustomValues } from "@/lib/rules";
import { Field } from "./ui";

export default function RuleInputs({ definitions, values = {}, onChange, disabled = false }: { definitions: RuleInput[]; values?: CustomValues; onChange: (values: CustomValues) => void; disabled?: boolean }) {
  function update(key: string, value: string | boolean) { const next = { ...values }; if (value === "") delete next[key]; else next[key] = value; onChange(next); }
  if (!definitions.length) return null;
  return <div className="policy-inputs"><div className="section-label">POLICY INPUTS</div>{definitions.map(field => <Field key={field.key} label={`${field.label || field.key}${field.required ? " *" : ""}`} hint={field.required ? "Required by this policy." : "Leave blank when the value is unknown."}>
    {field.type === "boolean" ? <select aria-label={field.label || field.key} value={values[field.key] === undefined ? "" : String(values[field.key])} disabled={disabled} onChange={e => update(field.key, e.target.value === "" ? "" : e.target.value === "true")}><option value="">Select an answer</option><option value="true">Yes</option><option value="false">No</option></select>
      : field.type === "enum" ? <select aria-label={field.label || field.key} value={String(values[field.key] ?? "")} disabled={disabled} onChange={e => update(field.key, e.target.value)}><option value="">Select a value</option>{(field.options || []).map(option => <option key={option} value={option}>{option}</option>)}</select>
        : <input aria-label={field.label || field.key} type={field.type === "date" ? "date" : "text"} inputMode={field.type === "money" || field.type === "number" ? "decimal" : undefined} value={String(values[field.key] ?? "")} disabled={disabled} onChange={e => update(field.key, e.target.value)} />}
  </Field>)}</div>;
}
