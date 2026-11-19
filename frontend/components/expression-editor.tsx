"use client";
import { Plus, Trash2 } from "lucide-react";
import type { ConfigData } from "@/lib/types";
import { Expression, expressionText, numberExpression, operatorNames, referenceExpression, referencesFor } from "@/lib/rules";
import { Field } from "./ui";

const comparisons = ["eq", "ne", "lt", "lte", "gt", "gte"];
const logic = ["and", "or", "not"];
export function defaultExpression(op: string): Expression {
  if (op === "number") return numberExpression();
  if (op === "string") return { op, value: "" };
  if (op === "pending") return { op, value: "Awaiting approved policy" };
  if (op === "boolean") return { op, value: "true" };
  if (op === "ref") return referenceExpression();
  if (op === "residual") return { op, ref: "inclusion" };
  if (op === "if") return { op, args: [{ op: "boolean", value: "true" }, numberExpression(), numberExpression()] };
  if (op === "lookup" || op === "schedule") return { op, args: [referenceExpression(op === "schedule" ? "input.ctcYear" : "input.gradeId"), op === "schedule" ? numberExpression("1") : { op: "string", value: "" }, numberExpression()] };
  if (op === "not") return { op, args: [{ op: "boolean", value: "true" }] };
  if (logic.includes(op)) return { op, args: [{ op: "boolean", value: "true" }, { op: "boolean", value: "true" }] };
  return { op, args: [referenceExpression(), numberExpression(op === "div" ? "2" : "0")] };
}

export function ExpressionEditor({ value, onChange, data, disabled = false, label = "Expression", condition = false, depth = 0 }: {
  value: Expression; onChange: (value: Expression) => void; data: ConfigData; disabled?: boolean; label?: string; condition?: boolean; depth?: number;
}) {
  const references = referencesFor(data);
  const args = value.args || [];
  function arg(index: number, next: Expression) { const changed = [...args]; changed[index] = next; onChange({ ...value, args: changed }); }
  const categories = [...new Set(references.map(r => r.category))];
  const options = condition ? ["eq", "ne", "lt", "lte", "gt", "gte", "and", "or", "not", "boolean", "ref", "if"] : Object.keys(operatorNames);
  const leaf = ["number", "string", "boolean", "ref", "residual", "pending"].includes(value.op);
  const table = value.op === "lookup" || value.op === "schedule";
  const mapLookup = value.op === "lookup" && !!value.ref;
  return <fieldset className={`expression-editor ${leaf ? "expression-leaf" : ""}`} disabled={disabled}>
    <legend>{label}</legend>
    <div className="expression-top">
      <Field label="Operation"><select aria-label={`${label} operation`} value={value.op} onChange={e => onChange(defaultExpression(e.target.value))}>
        {!options.includes(value.op) && <option value={value.op}>{operatorNames[value.op] || value.op}</option>}
        {options.map(op => <option key={op} value={op} disabled={depth >= 10 && !["number", "string", "boolean", "ref"].includes(op)}>{operatorNames[op]}</option>)}
      </select></Field>
      {value.op === "number" && <Field label="Value" hint="Use 0.12 for 12%."><input aria-label={`${label} value`} inputMode="decimal" value={value.value ?? ""} onChange={e => onChange({ op: "number", value: e.target.value })} /></Field>}
      {value.op === "string" && <Field label="Text"><input aria-label={`${label} text`} value={value.value || ""} onChange={e => onChange({ op: "string", value: e.target.value })} /></Field>}
      {value.op === "pending" && <Field label="What is missing?"><input aria-label={`${label} pending reason`} value={value.value || ""} onChange={e => onChange({ op: "pending", value: e.target.value })} /></Field>}
      {value.op === "boolean" && <Field label="Value"><select value={value.value || "false"} onChange={e => onChange({ op: "boolean", value: e.target.value })}><option value="true">Yes / true</option><option value="false">No / false</option></select></Field>}
      {value.op === "ref" && <Field label="Value source"><select aria-label={`${label} source`} value={value.ref || ""} onChange={e => onChange({ op: "ref", ref: e.target.value })}><option value="">Choose a source</option>{value.ref && !references.some(r => r.value === value.ref) && <option value={value.ref}>{value.ref}</option>}{categories.map(category => <optgroup key={category} label={category}>{references.filter(r => r.category === category).map(r => <option key={r.value} value={r.value}>{r.label} ({r.value})</option>)}</optgroup>)}</select></Field>}
      {value.op === "residual" && <Field label="Group to balance"><select aria-label={`${label} group`} value={value.ref || ""} onChange={e => onChange({ op: "residual", ref: e.target.value })}><option value="">Choose a group</option>{data.rules?.groups.filter(g => g.inGross).map(g => <option key={g.code} value={g.code}>{g.name || g.code}</option>)}</select></Field>}
    </div>
    {table && <>
      {value.op === "lookup" && <Field label="Lookup source"><select value={mapLookup ? "grade" : "table"} onChange={e => onChange(e.target.value === "grade" ? { op: "lookup", ref: "grade.professionalAllowance", args: [referenceExpression("input.qualification")] } : defaultExpression("lookup"))}><option value="table">Values defined below</option><option value="grade">Grade professional allowance slabs</option></select></Field>}
      <ExpressionEditor label={value.op === "schedule" ? "Schedule year" : "Lookup key"} value={args[0] || referenceExpression()} onChange={next => arg(0, next)} data={data} disabled={disabled} depth={depth + 1} />
      {!mapLookup && <div className="expression-table">{Array.from({ length: Math.floor((args.length - 1) / 2) }, (_, index) => <div className="expression-table-row" key={index}>
        <ExpressionEditor label={value.op === "schedule" ? `Year ${index + 1}` : `Match ${index + 1}`} value={args[index * 2 + 1]} onChange={next => arg(index * 2 + 1, next)} data={data} disabled={disabled} depth={depth + 1} />
        <ExpressionEditor label={`Result ${index + 1}`} value={args[index * 2 + 2]} onChange={next => arg(index * 2 + 2, next)} data={data} disabled={disabled} depth={depth + 1} />
        {!disabled && <button type="button" className="icon-button danger-hover" aria-label={`Remove lookup row ${index + 1}`} onClick={() => onChange({ ...value, args: args.filter((_, i) => i !== index * 2 + 1 && i !== index * 2 + 2) })}><Trash2 size={15} /></button>}
      </div>)}{!disabled && <button type="button" className="button button-ghost" onClick={() => onChange({ ...value, args: [...args, value.op === "schedule" ? numberExpression(String(Math.floor(args.length / 2) + 1)) : { op: "string", value: "" }, numberExpression()] })}><Plus size={15} />Add {value.op === "schedule" ? "year" : "lookup row"}</button>}</div>}
      <p className="field-hint">A missing lookup value stays pending. Add an explicit value for every supported case.</p>
    </>}
    {!leaf && !table && <div className="expression-arguments">{args.map((item, index) => <div className="expression-argument" key={index}>
      <ExpressionEditor value={item} onChange={next => arg(index, next)} data={data} disabled={disabled} depth={depth + 1} condition={(value.op === "if" && index === 0) || logic.includes(value.op)} label={value.op === "if" ? ["Condition", "Then", "Otherwise"][index] || `Value ${index + 1}` : logic.includes(value.op) ? `Condition ${index + 1}` : comparisons.includes(value.op) ? index === 0 ? "Left value" : "Right value" : `Value ${index + 1}`} />
      {!disabled && ["add", "mul", "min", "max", "and", "or"].includes(value.op) && args.length > 2 && <button type="button" className="icon-button danger-hover" aria-label={`Remove value ${index + 1}`} onClick={() => onChange({ ...value, args: args.filter((_, i) => i !== index) })}><Trash2 size={15} /></button>}
    </div>)}{!disabled && ["add", "mul", "min", "max", "and", "or"].includes(value.op) && <button type="button" className="button button-ghost" onClick={() => onChange({ ...value, args: [...args, logic.includes(value.op) ? { op: "boolean", value: "true" } : numberExpression()] })}><Plus size={15} />Add {logic.includes(value.op) ? "condition" : "value"}</button>}</div>}
    {depth === 0 && <p className="expression-summary">{expressionText(value)}</p>}
  </fieldset>;
}
