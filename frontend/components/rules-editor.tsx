"use client";
import TraceValues from "./trace-values";
import { useEffect, useRef, useState } from "react";
import { ChevronDown, FlaskConical, GitBranch, Plus, Trash2 } from "lucide-react";
import type { CalculationResult, CandidateInput, ConfigData, Configuration } from "@/lib/types";
import { demoInput } from "@/lib/types";
import { CalculationRule, Expression, RuleComponent, RuleInput, RuleSet, numberExpression, policyUsesInput, referenceExpression, resultGroupName, resultGroups, ruleReferences } from "@/lib/rules";
import { errorMessage, money, request } from "@/lib/api";
import { ExpressionEditor } from "./expression-editor";
import RuleInputs from "./rule-inputs";
import { Badge, Field, Notice, Spinner } from "./ui";

type Props = { configuration: Configuration; baseline?: Configuration; onChange: (rules: RuleSet) => void; editable: boolean; csrf: string; notify: (text: string, error?: boolean) => void };
const newRule = (): CalculationRule => ({ period: "monthly", status: "calculated", expression: numberExpression() });
function freshCode(prefix: string, existing: string[]): string { let n = 1; while (existing.includes(`${prefix}_${n}`)) n++; return `${prefix}_${n}`; }

export default function RulesEditor({ configuration, baseline, onChange, editable, csrf, notify }: Props) {
  const rules = configuration.data.rules!;
  const [section, setSection] = useState<"components" | "groups" | "inputs" | "dependencies" | "preview">("components");
  const [filter, setFilter] = useState("");
  const componentList = useRef<HTMLDivElement>(null);
  const addedComponent = useRef<string | null>(null);
  useEffect(() => {
    const code = addedComponent.current;
    if (!code) return;
    const editor = Array.from(componentList.current?.querySelectorAll<HTMLDetailsElement>("details.rule-component") || []).find(element => element.dataset.componentCode === code);
    if (!editor) return;
    // Reveal once after rendering; subsequent typing and manual collapse keep
    // the user's focus and expansion state.
    addedComponent.current = null;
    editor.open = true;
    editor.scrollIntoView({ block: "start", behavior: "auto" });
    const name = editor.querySelector<HTMLInputElement>('input[aria-label="Component name"]');
    name?.focus({ preventScroll: true });
    name?.select();
  }, [rules.components, filter]);
  function change(fn: (next: RuleSet) => void) { if (!editable) return; const next = structuredClone(rules); fn(next); onChange(next); }
  function addComponent() {
    if (!editable) return;
    const code = freshCode("COMP", rules.components.map(component => component.code));
    addedComponent.current = code;
    setFilter("");
    change(next => next.components.push({ code, name: "New component", group: next.groups[0]?.code || "", rules: [newRule()] }));
  }
  return <div className="config-stack rules-workspace">
    <Notice tone="blue">This version defines its calculations below. Saved revisions keep their original rules. Preview changes before saving and publishing.</Notice>
    <div className="rules-subnav" role="tablist" aria-label="Calculation rule sections">{([
      ["components", "Components"], ["groups", "Groups & targets"], ["inputs", "Policy inputs"], ["dependencies", "Dependencies"], ["preview", "Preview draft"],
    ] as const).filter(([id]) => editable || id !== "preview").map(([id, label]) => <button key={id} type="button" role="tab" aria-selected={section === id} className={section === id ? "active" : ""} onClick={() => setSection(id)}>{label}</button>)}</div>
    {section === "components" && <div className="card config-section">
      <div className="section-heading"><div><h2>Salary components <span className="count-badge">{rules.components.length}</span></h2><p>Choose how each component is calculated, when it applies, and where it appears.</p></div>{editable && <button type="button" className="button button-secondary" onClick={addComponent}><Plus size={16} />Add component</button>}</div>
      <Field label="Find a component"><input type="search" value={filter} placeholder="Search by name or code" onChange={e => setFilter(e.target.value)} /></Field>
      <div className="benefit-list" ref={componentList}>{rules.components.map((component, index) => !`${component.name} ${component.code}`.toLowerCase().includes(filter.toLowerCase()) ? null : <ComponentEditor key={index} component={component} data={configuration.data} editable={editable} canMoveUp={index > 0} canMoveDown={index < rules.components.length - 1} onMove={offset => change(r => { const target = index + offset; if (target >= 0 && target < r.components.length) [r.components[index], r.components[target]] = [r.components[target], r.components[index]]; })} onChange={next => change(r => { r.components[index] = next; })} onRemove={() => change(r => { r.components.splice(index, 1); })} />)}</div>
      {!rules.components.length && <p className="field-hint">Add the components this policy needs. Referenced components must exist before publication.</p>}
    </div>}
    {section === "groups" && <div className="card config-section">
      <div className="section-heading"><div><h2>Groups & targets</h2><p>Gross groups must reconcile exactly to Monthly Gross. Other groups are added to annual CTC.</p></div>{editable && <button type="button" className="button button-secondary" onClick={() => change(next => next.groups.push({ code: freshCode("group", next.groups.map(g => g.code)), name: "New group", inGross: false }))}><Plus size={16} />Add group</button>}</div>
      <div className="rules-card-list">{rules.groups.map((group, index) => <fieldset className="rules-group-card" key={index} disabled={!editable}>
        <legend>{group.name || `Group ${index + 1}`}</legend>
        <div className="field-grid"><Field label="Group code" hint="References use this code. Update dependent rules if you change it."><input aria-label="Group code" value={group.code} onChange={e => change(next => { next.groups[index].code = e.target.value; })} /></Field><Field label="Group name"><input value={group.name} onChange={e => change(next => { next.groups[index].name = e.target.value; })} /></Field></div>
        <label className="checkbox-row"><input type="checkbox" checked={group.inGross} onChange={e => change(next => { next.groups[index].inGross = e.target.checked; if (e.target.checked) next.groups[index].target = { op: "mul", args: [referenceExpression(), numberExpression("0.5")] }; else { delete next.groups[index].target; delete next.groups[index].balancer; } })} /><span>Included in Monthly Gross</span></label>
        {group.inGross ? <>
          <ExpressionEditor label="Group target" value={group.target || numberExpression()} data={configuration.data} disabled={!editable} onChange={value => change(next => { next.groups[index].target = value; })} />
          <Field label="Balancing component" hint="Select exactly one component in this group. Give it the Balance a group calculation."><select value={group.balancer || ""} onChange={e => change(next => { next.groups[index].balancer = e.target.value || undefined; })}><option value="">Choose a component</option>{group.balancer && !rules.components.some(c => c.group === group.code && c.code === group.balancer) && <option value={group.balancer}>{group.balancer} — missing or in another group</option>}{rules.components.filter(c => c.group === group.code).map(c => <option key={c.code} value={c.code}>{c.name} ({c.code})</option>)}</select></Field>
          <p className="field-hint">Targets may use candidate inputs, grade values, and configured parameters. Every Gross group needs a target; their targets must add up exactly to Gross.</p>
        </> : <p className="field-hint">Amounts in this group are additional CTC. Its components must not duplicate amounts already included in Gross.</p>}
        {editable && <div className="rules-row-actions"><button type="button" className="button button-ghost" disabled={index === 0} onClick={() => change(next => { [next.groups[index - 1], next.groups[index]] = [next.groups[index], next.groups[index - 1]]; })}>Move up</button><button type="button" className="button button-ghost" disabled={index === rules.groups.length - 1} onClick={() => change(next => { [next.groups[index + 1], next.groups[index]] = [next.groups[index], next.groups[index + 1]]; })}>Move down</button><button type="button" className="button button-danger-ghost" disabled={rules.components.some(c => c.group === group.code)} title={rules.components.some(c => c.group === group.code) ? "Move or remove this group's components first" : undefined} onClick={() => change(next => { next.groups.splice(index, 1); })}><Trash2 size={15} />Remove group</button></div>}
      </fieldset>)}</div>
    </div>}
    {section === "inputs" && <div className="card config-section">
      <div className="section-heading"><div><h2>Policy inputs</h2><p>Add information HR must provide for this policy. Existing Gross, Basic, grade, location, qualification, and job inputs remain available.</p></div>{editable && <button type="button" className="button button-secondary" onClick={() => change(next => next.inputs.push({ key: freshCode("field", next.inputs.map(i => i.key)), label: "New policy input", type: "money", required: true }))}><Plus size={16} />Add input</button>}</div>
      {!rules.inputs.length && <p className="field-hint">This policy uses only the existing candidate inputs.</p>}
      <div className="rules-card-list">{rules.inputs.map((field, index) => <fieldset className="rules-group-card" key={index} disabled={!editable}>
        <legend>{field.label || `Input ${index + 1}`}</legend>
        <div className="field-grid three-columns"><Field label="Input key" hint="Formulas use input.custom.key."><input aria-label="Input key" value={field.key} onChange={e => change(next => { next.inputs[index].key = e.target.value; })} /></Field><Field label="Label shown to HR"><input value={field.label} onChange={e => change(next => { next.inputs[index].label = e.target.value; })} /></Field><Field label="Value type"><select value={field.type} onChange={e => change(next => { next.inputs[index].type = e.target.value as RuleInput["type"]; if (e.target.value === "enum") next.inputs[index].options ||= []; else delete next.inputs[index].options; })}><option value="money">Money</option><option value="number">Number</option><option value="boolean">Yes / no</option><option value="enum">Choice list</option><option value="date">Date</option></select></Field></div>
        {field.type === "enum" && <Field label="Allowed choices" hint="One value per line. Formulas compare these exact values."><textarea rows={4} value={(field.options || []).join("\n")} onChange={e => change(next => { next.inputs[index].options = e.target.value.split("\n"); })} /></Field>}
        <label className="checkbox-row"><input type="checkbox" checked={field.required} onChange={e => change(next => { next.inputs[index].required = e.target.checked; })} />Required for calculations under this policy</label>
        {editable && <button type="button" className="button button-danger-ghost" onClick={() => change(next => { next.inputs.splice(index, 1); })}><Trash2 size={15} />Remove input</button>}
      </fieldset>)}</div>
    </div>}
    {section === "dependencies" && <div className="card config-section"><div className="section-heading"><div><h2>Calculation dependencies</h2><p>The engine derives calculation order from these references. Preview identifies missing references and circular dependencies.</p></div><GitBranch size={21} /></div><div className="table-scroll"><table><thead><tr><th>Component</th><th>Uses</th><th>Used by</th></tr></thead><tbody>{rules.components.map(c => { const refs = [...new Set(c.rules.flatMap(ruleReferences))]; const consumers = rules.components.filter(other => other.code !== c.code && other.rules.some(rule => ruleReferences(rule).includes(`component.${c.code}`))); return <tr key={c.code}><td><strong>{c.name}</strong><small className="cell-subtext">{c.code}</small></td><td>{refs.length ? refs.map(ref => <span className="dependency-chip" key={ref}>{ref}</span>) : <span className="muted">No dependencies</span>}</td><td>{consumers.map(other => other.name).join(", ") || "—"}</td></tr>; })}</tbody></table></div></div>}
    {section === "preview" && editable && <RulePreview configuration={configuration} baseline={baseline} csrf={csrf} notify={notify} />}
  </div>;
}

function ComponentEditor({ component, data, editable, canMoveUp, canMoveDown, onMove, onChange, onRemove }: { component: RuleComponent; data: ConfigData; editable: boolean; canMoveUp: boolean; canMoveDown: boolean; onMove: (offset: number) => void; onChange: (next: RuleComponent) => void; onRemove: () => void }) {
  function updateRule(index: number, next: CalculationRule) { onChange({ ...component, rules: component.rules.map((r, i) => i === index ? next : r) }); }
  return <details className="benefit-editor rule-component" data-component-code={component.code} style={{ scrollMarginTop: 24 }}><summary><span className="benefit-index">{component.code}</span><strong>{component.name || "Unnamed component"}</strong><Badge>{data.rules?.groups.find(g => g.code === component.group)?.name || component.group || "Choose group"}</Badge><ChevronDown size={16} /></summary>
    <fieldset className="benefit-editor-body" disabled={!editable}>
      <div className="field-grid three-columns"><Field label="Component code" hint="Changing a code requires updating its references."><input aria-label="Component code" value={component.code} onChange={e => onChange({ ...component, code: e.target.value })} /></Field><Field label="Component name"><input aria-label="Component name" value={component.name} onChange={e => onChange({ ...component, name: e.target.value })} /></Field><Field label="Group"><select value={component.group} onChange={e => onChange({ ...component, group: e.target.value })}><option value="">Select a group</option>{data.rules?.groups.map(g => <option key={g.code} value={g.code}>{g.name || g.code}</option>)}</select></Field></div>
      <p className="field-hint">Exactly one rule must match each case. Use mutually exclusive scope conditions for different grades, locations, or other inputs.</p>
      <div className="rules-card-list">{component.rules.map((rule, index) => <RuleEditor key={index} rule={rule} index={index} data={data} editable={editable} onChange={next => updateRule(index, next)} onRemove={() => onChange({ ...component, rules: component.rules.filter((_, i) => i !== index) })} />)}</div>
      <div className="rules-row-actions"><button type="button" className="button button-ghost" disabled={!editable || !canMoveUp} onClick={() => onMove(-1)}>Move up</button><button type="button" className="button button-ghost" disabled={!editable || !canMoveDown} onClick={() => onMove(1)}>Move down</button></div><p className="field-hint">Display order does not change the calculation order derived from dependencies. Components appear in this order within their groups.</p>
      {editable && <div className="rules-row-actions"><button type="button" className="button button-secondary" onClick={() => onChange({ ...component, rules: [...component.rules, { ...newRule(), when: { op: "boolean", value: "false" } }] })}><Plus size={15} />Add scoped rule</button><button type="button" className="button button-danger-ghost" onClick={onRemove}><Trash2 size={15} />Remove component</button></div>}
    </fieldset>
  </details>;
}

function RuleEditor({ rule, index, data, editable, onChange, onRemove }: { rule: CalculationRule; index: number; data: ConfigData; editable: boolean; onChange: (rule: CalculationRule) => void; onRemove: () => void }) {
  const status = rule.status || "calculated";
  function optional(key: "when" | "eligibility" | "min" | "max", next?: Expression) { const changed = { ...rule }; if (next) changed[key] = next; else delete changed[key]; onChange(changed); }
  return <div className="calculation-rule">
    <div className="section-heading"><h3>Rule {index + 1}</h3><Badge tone={status === "pending" ? "amber" : "blue"}>{status === "not_applicable" ? "Not applicable" : status}</Badge></div>
    <div className="field-grid three-columns"><Field label="Rule reference" hint="Optional name for the calculation trace."><input aria-label="Rule reference" value={rule.id || ""} onChange={e => onChange({ ...rule, id: e.target.value || undefined })} /></Field><Field label="Treatment"><select value={status} onChange={e => onChange({ ...rule, status: e.target.value as CalculationRule["status"], ...(e.target.value === "calculated" && !rule.expression ? { expression: numberExpression() } : {}) })}><option value="calculated">Calculate amount</option><option value="not_applicable">Not applicable — confirmed zero</option><option value="pending">Pending policy — unresolved</option></select></Field><Field label="Payment period"><select value={rule.period} onChange={e => onChange({ ...rule, period: e.target.value as CalculationRule["period"] })}><option value="">Not configured</option><option value="monthly">Monthly</option><option value="annual">Annual</option><option value="one_time">One-time</option></select></Field></div>
    <OptionalExpression label="Apply this rule when" description="Scope selects the rule. Overlapping matches are rejected." value={rule.when} onChange={v => optional("when", v)} data={data} editable={editable} condition />
    <OptionalExpression label="Eligibility condition" description="An ineligible candidate receives an explicit zero. Missing eligibility stays pending." value={rule.eligibility} onChange={v => optional("eligibility", v)} data={data} editable={editable} condition />
    {status === "calculated" && <ExpressionEditor label="Amount calculation" value={rule.expression || numberExpression()} data={data} disabled={!editable} onChange={expression => onChange({ ...rule, expression })} />}
    {status === "calculated" && <details className="rule-limits"><summary>Validation limits<ChevronDown size={15} /></summary><p className="field-hint">These limits reject out-of-range values. To cap a payment, use Smaller value / cap in its amount calculation.</p><label className="checkbox-row"><input type="checkbox" checked={!!rule.limitsTogether} onChange={e => onChange({ ...rule, limitsTogether: e.target.checked })} />Validate minimum and maximum together</label><p className="field-hint">If either configured limit is missing, validation stays pending.</p><OptionalExpression label="Minimum allowed amount" value={rule.min} onChange={v => optional("min", v)} data={data} editable={editable} /><OptionalExpression label="Maximum allowed amount" value={rule.max} onChange={v => optional("max", v)} data={data} editable={editable} /></details>}
    <Field label="Policy reference / notes"><textarea rows={2} value={rule.notes || ""} onChange={e => onChange({ ...rule, notes: e.target.value || undefined })} /></Field>
    {editable && <button type="button" className="button button-danger-ghost" onClick={onRemove}><Trash2 size={15} />Remove rule {index + 1}</button>}
  </div>;
}

function OptionalExpression({ label, description, value, onChange, data, editable, condition = false }: { label: string; description?: string; value?: Expression; onChange: (v?: Expression) => void; data: ConfigData; editable: boolean; condition?: boolean }) {
  return <div className="optional-expression"><label className="checkbox-row"><input type="checkbox" checked={!!value} disabled={!editable} onChange={e => onChange(e.target.checked ? condition ? { op: "eq", args: [referenceExpression("input.gradeId"), { op: "string", value: "" }] } : numberExpression() : undefined)} />{label}</label>{description && <p className="field-hint">{description}</p>}{value && <ExpressionEditor label={label} value={value} onChange={onChange} data={data} disabled={!editable} condition={condition} />}</div>;
}

function RulePreview({ configuration, baseline, csrf, notify }: { configuration: Configuration; baseline?: Configuration; csrf: string; notify: Props["notify"] }) {
  const [input, setInput] = useState<CandidateInput>(() => ({ ...structuredClone(demoInput), candidateName: "Policy preview", gradeId: configuration.data.grades[0]?.id || "", locationId: configuration.data.locations[0]?.id || "", calculationDate: configuration.effectiveFrom || "", custom: {} }));
  const [result, setResult] = useState<CalculationResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [stale, setStale] = useState(false);
  const [compare, setCompare] = useState(false);
  const [previous, setPrevious] = useState<CalculationResult | null>(null);
  const [comparisonError, setComparisonError] = useState("");
  const previewKey = JSON.stringify({ configuration, input, compare });
  const latestPreviewKey = useRef(previewKey); latestPreviewKey.current = previewKey;
  useEffect(() => { setStale(true); }, [previewKey]);
  function update<K extends keyof CandidateInput>(key: K, value: CandidateInput[K]) { setInput(current => ({ ...current, [key]: value })); }
  async function run() {
    const requestedKey = previewKey;
    setBusy(true); setError(""); setPrevious(null); setComparisonError("");
    try {
      const preview = await request<CalculationResult>("/configurations/preview", { method: "POST", csrf, body: { configuration, input } }); setResult(preview);
      if (compare && baseline?.id) {
        const allowed = new Set((baseline.data.rules?.inputs || []).map(i => i.key));
        const previousInput = { ...input, custom: Object.fromEntries(Object.entries(input.custom || {}).filter(([key]) => allowed.has(key))) };
        try { setPrevious(await request<CalculationResult>("/calculate", { method: "POST", csrf, body: { configurationId: baseline.id, input: previousInput } })); } catch (e) { setComparisonError(errorMessage(e)); }
      }
      setStale(latestPreviewKey.current !== requestedKey);
    } catch (e) { setResult(null); setError(errorMessage(e)); notify("Draft preview needs attention. Review the validation message.", true); } finally { setBusy(false); }
  }
  return <div className="card config-section"><div className="section-heading"><div><h2>Preview draft calculations</h2><p>Try this unsaved configuration with sample inputs. Preview creates no candidate record.</p></div><FlaskConical size={21} /></div>
    <fieldset disabled={busy} className="preview-inputs"><div className="field-grid three-columns">
      <Field label="Preview monthly gross"><input inputMode="decimal" value={input.gross} onChange={e => update("gross", e.target.value)} /></Field>{(policyUsesInput(configuration.data, "basic") || compare && policyUsesInput(baseline?.data, "basic")) && <Field label="Preview Basic salary"><input inputMode="decimal" value={input.basic} onChange={e => update("basic", e.target.value)} /></Field>}<Field label="Preview calculation date"><input type="date" value={input.calculationDate || ""} onChange={e => update("calculationDate", e.target.value)} /></Field>
      <Field label="Preview grade"><select value={input.gradeId} onChange={e => update("gradeId", e.target.value)}><option value="">Choose grade</option>{configuration.data.grades.map(g => <option key={g.id} value={g.id}>{g.name}</option>)}</select></Field><Field label="Preview location"><select value={input.locationId} onChange={e => update("locationId", e.target.value)}><option value="">Choose location</option>{configuration.data.locations.map(l => <option key={l.id} value={l.id}>{l.name}</option>)}</select></Field><Field label="Preview qualification"><select value={input.qualification} onChange={e => update("qualification", e.target.value as CandidateInput["qualification"])}>{["NONE", "CA", "CMA", "CS"].map(q => <option key={q} value={q}>{q}</option>)}</select></Field>
      <Field label="Preview tax regime"><select value={input.taxRegime} onChange={e => update("taxRegime", e.target.value as "new" | "old")}><option value="new">New</option><option value="old">Old</option></select></Field><Field label="Preview CTC year"><input type="number" min={1} value={input.ctcYear} onChange={e => update("ctcYear", Number(e.target.value))} /></Field><Field label="Preview joining date"><input type="date" value={input.joiningDate} onChange={e => update("joiningDate", e.target.value)} /></Field>
    </div><div className="preview-flags"><label className="checkbox-row"><input type="checkbox" checked={input.npsOptIn} onChange={e => update("npsOptIn", e.target.checked)} />NPS opted in</label>{Object.keys(input.job).map(key => <label key={key} className="checkbox-row"><input type="checkbox" checked={input.job[key as keyof CandidateInput["job"]]} onChange={e => update("job", { ...input.job, [key]: e.target.checked })} />{key}</label>)}</div><RuleInputs definitions={configuration.data.rules?.inputs || []} values={input.custom} onChange={value => update("custom", value)} disabled={busy} /></fieldset>
    {baseline?.id && <label className="checkbox-row preview-compare"><input type="checkbox" checked={compare} disabled={busy} onChange={e => setCompare(e.target.checked)} />Compare with saved base version {baseline.version} using these inputs</label>}
    <button type="button" className="button button-primary" onClick={run} disabled={busy}>{busy ? <Spinner label="Calculating preview" /> : <><FlaskConical size={16} />Run preview</>}</button>
    {error && <div className="preview-feedback"><Notice tone="red">{error}</Notice></div>}
    {result && <div className="preview-feedback">{stale && <Notice>Inputs or rules have changed. Run preview again to update these results.</Notice>}<Notice tone={result.coreValid ? "green" : "red"}>{result.coreValid ? "Group targets and core calculations reconcile." : "This draft has calculation errors or unresolved core rules."}</Notice>
      <div className="rules-result-groups">{resultGroups(result).map(g => <div className="rule-result-group" key={g.code}><strong>{g.name}</strong><span>{money(g.monthly)} / month</span>{g.inGross && <small>Target {money(g.target)} · Difference {money(g.difference)}</small>}{!g.inGross && <small>Annual {money(g.annual)}</small>}</div>)}</div>
      <div className="table-scroll"><table><thead><tr><th>Component</th><th>Group</th><th>Monthly</th><th>Annual</th></tr></thead><tbody>{result.lines.map(line => <tr key={line.code}><td><strong>{line.name}</strong><details className="preview-trace"><summary>Calculation details</summary><p>{line.formula}</p>{result.trace?.filter(t => t.component === line.code).map((trace, index) => <p key={index}>Rule {trace.rule || "—"}: {trace.expression}<br />Uses: {(trace.dependencies || []).join(", ") || "No dependencies"}<TraceValues values={trace.values} /></p>)}</details></td><td>{resultGroupName(result, line.group)}</td><td>{line.status === "pending" ? <Badge tone="amber">Pending</Badge> : money(line.monthly)}</td><td>{line.status === "pending" ? "—" : money(line.annual)}</td></tr>)}</tbody></table></div>
      <p className="preview-total"><strong>{result.totals.annualCTC ? "Annual CTC" : "Known annual subtotal"}: {money(result.totals.annualCTC || result.totals.knownAnnualTotal)}</strong></p>
      {comparisonError && <Notice>Saved-version comparison could not complete: {comparisonError}</Notice>}
      {previous && <div className="preview-comparison"><h3>Changes from saved version {baseline?.version}</h3><div className="table-scroll"><table><thead><tr><th>Component</th><th>Saved monthly</th><th>Draft monthly</th><th>Monthly change</th><th>Saved annual</th><th>Draft annual</th><th>Annual change</th></tr></thead><tbody>{[...new Set([...previous.lines.map(l => l.code), ...result.lines.map(l => l.code)])].map(code => { const before = previous.lines.find(l => l.code === code); const after = result.lines.find(l => l.code === code); return <tr key={code}><td>{after?.name || before?.name || code}{!before ? " (added)" : !after ? " (removed)" : ""}</td><td>{before?.status === "pending" ? "Pending" : money(before?.monthly)}</td><td>{after?.status === "pending" ? "Pending" : money(after?.monthly)}</td><td>{decimalDifference(after?.monthly, before?.monthly)}</td><td>{before?.status === "pending" ? "Pending" : money(before?.annual)}</td><td>{after?.status === "pending" ? "Pending" : money(after?.annual)}</td><td>{decimalDifference(after?.annual, before?.annual)}</td></tr>; })}</tbody></table></div></div>}
      {!!result.issues.length && <ul className="issue-list">{result.issues.map((issue, index) => <li key={index} className={`issue-${issue.severity}`}><span className="issue-dot" /><div><strong>{issue.field || issue.code}</strong><p>{issue.message}</p></div></li>)}</ul>}
    </div>}
  </div>;
}

// Difference is display-only; use decimal strings and integers to avoid float drift.
function decimalDifference(after?: string | null, before?: string | null): string {
  if (after == null || before == null || !/^-?\d+(\.\d+)?$/.test(after) || !/^-?\d+(\.\d+)?$/.test(before)) return "—";
  const scale = Math.max(after.split(".")[1]?.length || 0, before.split(".")[1]?.length || 0);
  const integer = (value: string) => { const sign = value.startsWith("-") ? -1n : 1n; const [whole, fraction = ""] = value.replace(/^-/, "").split("."); return sign * BigInt(whole + fraction.padEnd(scale, "0")); };
  const difference = integer(after) - integer(before); const absolute = (difference < 0n ? -difference : difference).toString().padStart(scale + 1, "0");
  const value = `${difference < 0n ? "-" : ""}${scale ? `${absolute.slice(0, -scale)}.${absolute.slice(-scale)}` : absolute}`;
  return `${difference > 0n ? "+" : ""}${money(value)}`;
}
