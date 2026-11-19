import type { CalculationResult, ConfigData } from "./types";

export interface Expression { op: string; value?: string; ref?: string; args?: Expression[] }
export interface CalculationRule {
  id?: string; when?: Expression; eligibility?: Expression; expression?: Expression;
  period: "" | "monthly" | "annual" | "one_time";
  status?: "calculated" | "not_applicable" | "pending";
  min?: Expression; max?: Expression; limitsTogether?: boolean; notes?: string;
}
export interface RuleComponent { code: string; name: string; group: string; rules: CalculationRule[] }
export interface RuleGroup { code: string; name: string; inGross: boolean; target?: Expression; balancer?: string }
export interface RuleInput { key: string; label: string; type: "money" | "number" | "boolean" | "enum" | "date"; required: boolean; options?: string[] }
export interface RuleSet { version: 2; groups: RuleGroup[]; components: RuleComponent[]; inputs: RuleInput[] }
export interface ResultGroup { code: string; name: string; inGross: boolean; target?: string | null; monthly?: string | null; annual?: string | null; difference?: string | null; valid: boolean }
export interface RuleTrace { component: string; rule: string; expression: string; dependencies: string[]; values?: Record<string, string | number | boolean | null> }
export type CustomValues = Record<string, string | number | boolean>;
export const numberExpression = (value = "0"): Expression => ({ op: "number", value });
export const referenceExpression = (ref = "input.gross"): Expression => ({ op: "ref", ref });
export const operatorNames: Record<string, string> = {
  number: "Number / fixed amount", string: "Text value", boolean: "Yes / no value", ref: "Use an input or calculated value",
  add: "Add", sub: "Subtract", mul: "Multiply / percentage", div: "Divide", min: "Smaller value / cap", max: "Larger value / floor",
  if: "If / then / otherwise", eq: "Equals", ne: "Does not equal", lt: "Less than", lte: "At most", gt: "Greater than", gte: "At least",
  and: "All conditions (AND)", or: "Any condition (OR)", not: "Not", lookup: "Master data lookup", schedule: "Annual schedule", residual: "Balance a group", pending: "Awaiting policy / input",
};
export function expressionText(expr?: Expression): string {
  if (!expr) return "Not configured";
  if (expr.op === "ref") return expr.ref || "Choose a value";
  if (["number", "string", "boolean"].includes(expr.op)) return expr.op === "string" ? `“${expr.value || ""}”` : expr.value || "0";
  if (expr.op === "residual") return `Target of ${expr.ref || "group"} − other components in that group`;
  if (expr.op === "pending") return `Pending: ${expr.value || "Policy not confirmed"}`;
  const args = (expr.args || []).map(expressionText);
  const signs: Record<string, string> = { add: "+", sub: "−", mul: "×", div: "÷", eq: "=", ne: "≠", lt: "<", lte: "≤", gt: ">", gte: "≥", and: "AND", or: "OR" };
  if (signs[expr.op]) return `(${args.join(` ${signs[expr.op]} `)})`;
  if (expr.op === "if") return `If ${args[0] || "…"}, then ${args[1] || "…"}, otherwise ${args[2] || "…"}`;
  if (expr.op === "not") return `Not (${args[0] || "…"})`;
  return `${expr.op}${expr.ref ? ` ${expr.ref}` : ""}(${args.join(", ")})`;
}
export function expressionReferences(expr?: Expression): string[] {
  if (!expr) return [];
  return [...new Set([...(expr.op === "ref" && expr.ref ? [expr.ref] : []), ...(expr.op === "residual" && expr.ref ? [`target.${expr.ref}`, `other components in ${expr.ref}`] : []), ...(expr.args || []).flatMap(expressionReferences)])];
}
export function ruleReferences(rule: CalculationRule): string[] {
  return [...new Set([rule.when, rule.eligibility, rule.expression, rule.min, rule.max].flatMap(expressionReferences))];
}
export function referencesFor(data: ConfigData): { value: string; label: string; category: string }[] {
  const values = [
    ["input.gross", "Monthly gross"], ["input.basic", "Entered Basic salary"], ["input.gradeId", "Grade code"], ["input.locationId", "Location code"],
    ["input.qualification", "Professional qualification"], ["input.taxRegime", "Tax regime"], ["input.npsOptIn", "NPS opt-in"], ["input.ctcYear", "CTC year"],
    ["input.calculationDate", "Calculation effective date"], ["input.joiningDate", "Joining date"],
    ...["sixthDay", "fieldService", "relocation", "deputation", "additionalHra"].map(key => [`input.job.${key}`, `Job: ${key}`]),
  ].map(([value, label]) => ({ value, label, category: "Candidate inputs" }));
  values.push(...(data.rules?.inputs || []).map(i => ({ value: `input.custom.${i.key}`, label: i.label || i.key, category: "Policy inputs" })));
  values.push(...["basicMin", "basicMax", "conveyance", "conveyanceCap"].map(key => ({ value: `grade.${key}`, label: `Grade ${key}`, category: "Grade matrix" })));
  values.push(...["pf.basicThreshold", "pf.inclusionShare", "pf.basisCap", "pf.rate", "nps.newRate", "nps.oldRate", "esi.rate", "esi.ceiling", "esi.ceilingInclusive", "esi.eligibilityBasis", "esi.contributionBasis", "gratuityRate", "basicMethodConfirmed", "rounding.confirmed", "annualizationConfirmed", "hraApproved", "templateApproved"].map(key => ({ value: `config.${key}`, label: key, category: "Configured parameters" })));
  values.push(...(data.rules?.components || []).map(c => ({ value: `component.${c.code}`, label: c.name || c.code, category: "Calculated components" })));
  for (const g of data.rules?.groups || []) {
    if (g.target) values.push({ value: `target.${g.code}`, label: `${g.name} target`, category: "Group targets and totals" });
    values.push({ value: `total.${g.code}`, label: `${g.name} total`, category: "Group targets and totals" });
  }
  return values;
}
const legacyNames: Record<string, string> = { inclusion: "Inclusion wages", exclusion: "Exclusion wages", statutory: "Statutory payments", policy: "Policy-based payments", variable: "Variable pay" };
export function resultGroups(result: CalculationResult): ResultGroup[] {
  if (result.groups?.length) return result.groups;
  return Object.entries(legacyNames).map(([code, name]) => ({ code, name, inGross: code === "inclusion" || code === "exclusion", valid: result.coreValid,
    monthly: code === "inclusion" ? result.totals.inclusion : code === "exclusion" ? result.totals.exclusion : undefined,
    annual: code === "statutory" ? result.totals.statutoryAnnual : code === "policy" ? result.totals.policyAnnual : code === "variable" ? result.totals.variableAnnual : undefined }));
}
export function resultGroupName(result: CalculationResult, code: string): string { return result.groups?.find(g => g.code === code)?.name || legacyNames[code] || code; }
export function policyUsesInput(data: ConfigData | undefined, key: string): boolean {
  if (!data?.rules) return true;
  return data.rules.groups.some(g => expressionReferences(g.target).includes(`input.${key}`)) || data.rules.components.some(c => c.rules.some(r => ruleReferences(r).includes(`input.${key}`)));
}
