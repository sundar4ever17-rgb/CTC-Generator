package engine

import (
	"encoding/json"
	"fmt"
)

func ruleNumber(v string) Expression                  { return Expression{Op: "number", Value: v} }
func ruleString(v string) Expression                  { return Expression{Op: "string", Value: v} }
func ruleRef(v string) Expression                     { return Expression{Op: "ref", Ref: v} }
func ruleOp(op string, args ...Expression) Expression { return Expression{Op: op, Args: args} }
func expressionPointer(v Expression) *Expression      { return &v }

// ConvertConfiguration creates a detached draft. It never changes the source
// configuration, its published rules, or any saved calculation.
func ConvertConfiguration(c Configuration) Configuration {
	var out Configuration
	encoded, _ := json.Marshal(c)
	_ = json.Unmarshal(encoded, &out)
	out.ID, out.CreatedAt, out.Version, out.Status = "", "", 0, "draft"
	if out.Data.Rules != nil {
		return out
	}
	r := &RuleSet{Version: 2, Inputs: []InputDefinition{}}
	for _, group := range []struct{ code, name, balancer string }{{"inclusion", "Inclusion wages", "SPECIAL"}, {"exclusion", "Exclusion wages", "HRA"}, {"statutory", "Statutory payments", ""}, {"policy", "Policy-based payments", ""}, {"variable", "Variable pay", ""}} {
		g := GroupDefinition{Code: group.code, Name: group.name, InGross: group.balancer != "", Balancer: group.balancer}
		if g.InGross {
			g.Target = expressionPointer(ruleOp("div", ruleRef("input.gross"), ruleNumber("2")))
		}
		r.Groups = append(r.Groups, g)
	}
	for _, old := range Components {
		component := ComponentDefinition{Code: old.Code, Name: old.Name, Group: old.Group, Rules: []CalculationRule{}}
		if old.Code == "ANNUAL_SPECIAL" && c.Data.AnnualSpecialGroup == "policy" {
			component.Group = "policy"
		}
		rule := CalculationRule{ID: old.Code + "_default", Period: "monthly", Status: "calculated"}
		var expr Expression
		switch old.Code {
		case "BASIC":
			expr = ruleRef("input.basic")
			rule.LimitsTogether = true
			rule.Min = expressionPointer(ruleRef("grade.basicMin"))
			rule.Max = expressionPointer(ruleRef("grade.basicMax"))
		case "PROFESSIONAL":
			rule.Eligibility = expressionPointer(ruleOp("ne", ruleRef("input.qualification"), ruleString("NONE")))
			expr = Expression{Op: "lookup", Ref: "grade.professionalAllowance", Args: []Expression{ruleRef("input.qualification")}}
		case "CONVEYANCE":
			expr = ruleRef("grade.conveyance")
			rule.Max = expressionPointer(ruleRef("grade.conveyanceCap"))
		case "PF":
			expr = ruleOp("if", ruleOp("gt", ruleRef("component.BASIC"), ruleRef("config.pf.basicThreshold")), ruleOp("mul", ruleRef("component.BASIC"), ruleRef("config.pf.rate")), ruleOp("mul", ruleOp("min", ruleOp("mul", ruleRef("target.inclusion"), ruleRef("config.pf.inclusionShare")), ruleRef("config.pf.basisCap")), ruleRef("config.pf.rate")))
		case "NPS":
			rule.Eligibility = expressionPointer(ruleRef("input.npsOptIn"))
			expr = ruleOp("mul", ruleRef("component.BASIC"), ruleOp("if", ruleOp("eq", ruleRef("input.taxRegime"), ruleString("old")), ruleRef("config.nps.oldRate"), ruleRef("config.nps.newRate")))
		case "SPECIAL", "HRA":
			expr = Expression{Op: "residual", Ref: component.Group}
		case "ESI":
			basis := func(field string) Expression {
				return ruleOp("lookup", ruleRef("config.esi."+field), ruleString("basic"), ruleRef("component.BASIC"), ruleString("gross"), ruleRef("input.gross"), ruleString("inclusion"), ruleRef("target.inclusion"), ruleString("exclusion"), ruleRef("target.exclusion"))
			}
			// The legacy ESI rule requires an explicit contribution wage basis even
			// when the candidate ultimately falls outside the eligibility ceiling.
			rule.When = expressionPointer(ruleOp("eq", basis("contributionBasis"), basis("contributionBasis")))
			rule.Eligibility = expressionPointer(ruleOp("if", ruleRef("config.esi.ceilingInclusive"), ruleOp("lte", basis("eligibilityBasis"), ruleRef("config.esi.ceiling")), ruleOp("lt", basis("eligibilityBasis"), ruleRef("config.esi.ceiling"))))
			expr = ruleOp("mul", basis("contributionBasis"), ruleRef("config.esi.rate"))
		case "GRATUITY":
			expr = ruleOp("mul", ruleRef("target.inclusion"), ruleRef("config.gratuityRate"))
		default:
			for i, benefit := range c.Data.Benefits {
				if benefit.Component != old.Code {
					continue
				}
				br := CalculationRule{ID: fmt.Sprintf("%s_%d", old.Code, i+1), Period: benefit.Period, Status: "calculated", Notes: benefit.Notes}
				var scopes []Expression
				if benefit.GradeID != "" {
					scopes = append(scopes, ruleOp("eq", ruleRef("input.gradeId"), ruleString(benefit.GradeID)))
				}
				if benefit.LocationID != "" {
					scopes = append(scopes, ruleOp("eq", ruleRef("input.locationId"), ruleString(benefit.LocationID)))
				}
				if len(scopes) == 1 {
					br.When = expressionPointer(scopes[0])
				} else if len(scopes) > 1 {
					br.When = expressionPointer(ruleOp("and", scopes...))
				}
				var conditions []Expression
				for _, condition := range benefit.Conditions {
					conditions = append(conditions, ruleOp("eq", ruleRef("input.job."+condition.Field), Expression{Op: "boolean", Value: fmt.Sprint(condition.Equals)}))
				}
				if len(conditions) == 1 {
					br.Eligibility = expressionPointer(conditions[0])
				} else if len(conditions) > 1 {
					br.Eligibility = expressionPointer(ruleOp("and", conditions...))
				}
				var amount Expression
				switch benefit.Kind {
				case "pending":
					br.Status = "pending"
				case "none":
					br.Status = "not_applicable"
				case "fixed":
					amount = ruleNumber(benefit.Amount)
				case "percent":
					refs := map[string]string{"basic": "component.BASIC", "gross": "input.gross", "inclusion": "target.inclusion", "exclusion": "target.exclusion", "hra": "component.HRA"}
					amount = ruleOp("mul", ruleRef(refs[benefit.Basis]), ruleNumber(benefit.Rate))
					if benefit.BasisPeriod == "annual" {
						amount = ruleOp("if", ruleRef("config.annualizationConfirmed"), ruleOp("mul", amount, ruleNumber("12")), Expression{Op: "pending", Value: "Annual percentage basis requires annualization confirmation"})
					}
				case "schedule":
					amount = Expression{Op: "schedule", Args: []Expression{ruleRef("input.ctcYear")}}
					for _, entry := range benefit.Schedule {
						amount.Args = append(amount.Args, ruleNumber(fmt.Sprint(entry.Year)), ruleNumber(entry.Amount))
					}
				default:
					br.Status = "pending"
				}
				if br.Status == "calculated" {
					if benefit.Cap != "" {
						amount = ruleOp("min", amount, ruleNumber(benefit.Cap))
					}
					br.Expression = expressionPointer(amount)
				}
				component.Rules = append(component.Rules, br)
			}
			if len(component.Rules) == 0 {
				component.Rules = append(component.Rules, CalculationRule{ID: old.Code + "_pending", Status: "pending", Notes: "Client rule has not been supplied"})
			}
			r.Components = append(r.Components, component)
			continue
		}
		rule.Expression = expressionPointer(expr)
		component.Rules = append(component.Rules, rule)
		r.Components = append(r.Components, component)
	}
	out.Data.Rules = r
	return out
}
