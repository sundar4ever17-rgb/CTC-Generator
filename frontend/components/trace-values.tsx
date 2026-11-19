import { money } from "@/lib/api";
import type { RuleTrace } from "@/lib/rules";

function displayValue(ref: string, value: string | number | boolean | null): string {
  if (value === null) return "Not available";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  const monetary = /^(component|target|total)\./.test(ref) || /^input\.(gross|basic)$/.test(ref) || /^grade\.(basicMin|basicMax|conveyance|conveyanceCap)$/.test(ref) || /^config\.(pf\.(basicThreshold|basisCap)|esi\.ceiling)$/.test(ref);
  return monetary && /^-?\d+(\.\d+)?$/.test(String(value)) ? money(String(value)) : String(value);
}
export default function TraceValues({ values }: { values?: RuleTrace["values"] }) {
  if (!values || !Object.keys(values).length) return null;
  return <span className="trace-values"><strong>Resolved values</strong>{Object.entries(values).map(([ref, value]) => <span className="trace-value" key={ref}><code>{ref}</code><span>{displayValue(ref, value)}</span></span>)}</span>;
}
