import { ConfigData } from "./types";

// Empty editing structure only. No grade, rate, ceiling or policy is assumed.
export function emptyConfiguration(): ConfigData {
  return {
    grades: [], locations: [], basicMethod: "manual", basicMethodConfirmed: false,
    pf: { basicThreshold: "", inclusionShare: "", basisCap: "", rate: "" },
    nps: { oldRate: "", newRate: "" },
    esi: { rate: "", ceiling: null, eligibilityBasis: "", contributionBasis: "", ceilingInclusive: null },
    gratuityRate: "", benefits: [], rounding: { scale: 2, mode: "half_up", confirmed: false },
    annualizationConfirmed: false, hraApproved: false, templateApproved: false,
    annualSpecialGroup: "statutory", offerTemplate: { companyName: "", address: "", signatory: "", body: "" },
  };
}

export function isEditableConfiguration(value: unknown): value is ConfigData {
  const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
  const strings = (v: unknown, keys: string[]) => object(v) && keys.every(k => typeof v[k] === "string");
  if (!object(value) || !Array.isArray(value.grades) || !Array.isArray(value.locations) || !Array.isArray(value.benefits)) return false;
  if (value.rules !== undefined) {
    const expression = (v: unknown, depth = 0): boolean => object(v) && depth <= 24 && typeof v.op === "string" && (v.value === undefined || typeof v.value === "string") && (v.ref === undefined || typeof v.ref === "string") && (v.args === undefined || Array.isArray(v.args) && v.args.every(a => expression(a, depth + 1)));
    const rules = value.rules;
    if (!object(rules) || rules.version !== 2 || !Array.isArray(rules.groups) || !Array.isArray(rules.components) || !Array.isArray(rules.inputs)) return false;
    if (!rules.groups.every(g => strings(g, ["code", "name"]) && object(g) && typeof g.inGross === "boolean" && (g.target === undefined || expression(g.target)) && (g.balancer === undefined || typeof g.balancer === "string"))) return false;
    if (!rules.components.every(c => strings(c, ["code", "name", "group"]) && object(c) && Array.isArray(c.rules) && c.rules.every(r => object(r) && ["", "monthly", "annual", "one_time"].includes(String(r.period)) && ["when", "eligibility", "expression", "min", "max"].every(key => r[key] === undefined || expression(r[key]))))) return false;
    if (!rules.inputs.every(i => strings(i, ["key", "label", "type"]) && object(i) && ["money", "number", "boolean", "enum", "date"].includes(String(i.type)) && typeof i.required === "boolean" && (i.options === undefined || Array.isArray(i.options) && i.options.every(o => typeof o === "string")))) return false;
  }
  if (!strings(value.pf, ["basicThreshold", "inclusionShare", "basisCap", "rate"]) || !strings(value.nps, ["oldRate", "newRate"]) || !strings(value.esi, ["rate", "eligibilityBasis", "contributionBasis"]) || !strings(value.offerTemplate, ["companyName", "address", "signatory", "body"])) return false;
  if (!object(value.rounding) || typeof value.rounding.scale !== "number" || typeof value.rounding.mode !== "string" || typeof value.rounding.confirmed !== "boolean" || typeof value.gratuityRate !== "string") return false;
  if (!["basicMethodConfirmed", "annualizationConfirmed", "hraApproved", "templateApproved"].every(k => typeof value[k] === "boolean")) return false;
  if (!value.grades.every(g => strings(g, ["id", "name"]) && object(g) && object(g.professionalAllowance) && Object.values(g.professionalAllowance).every(v => typeof v === "string") && ["basicMin", "basicMax", "conveyance", "conveyanceCap"].every(k => g[k] == null || typeof g[k] === "string"))) return false;
  if (!value.locations.every(l => strings(l, ["id", "name"]))) return false;
  return value.benefits.every(b => strings(b, ["component", "kind", "period"]) && object(b) &&
    (b.conditions === undefined || Array.isArray(b.conditions) && b.conditions.every(c => strings(c, ["field"]) && object(c) && typeof c.equals === "boolean")) &&
    (b.schedule === undefined || Array.isArray(b.schedule) && b.schedule.every(s => strings(s, ["amount"]) && object(s) && typeof s.year === "number")));
}
