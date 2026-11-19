package engine

import (
	"encoding/json"
	"strings"
)

// staticRuleValue evaluates only branches determined by supplied master data.
// Candidate-dependent values remain unknown rather than acquiring test amounts.
func staticRuleValue(x *ruleEvaluator, e *Expression) (ruleValue, bool) {
	if e == nil {
		return ruleValue{}, false
	}
	if e.Op == "ref" {
		if strings.HasPrefix(e.Ref, "config.") || strings.HasPrefix(e.Ref, "grade.") || e.Ref == "input.gradeId" || e.Ref == "input.qualification" || (e.Ref == "input.locationId" && x.in.LocationID != "") {
			return x.referenceValue(e.Ref), true
		}
		return ruleValue{}, false
	}
	if e.Op == "residual" {
		return ruleValue{}, false
	}
	if e.Op == "if" {
		condition, known := staticRuleValue(x, &e.Args[0])
		if !known {
			return ruleValue{}, false
		}
		if !condition.known {
			return condition, true
		}
		branch := 2
		if condition.b {
			branch = 1
		}
		return staticRuleValue(x, &e.Args[branch])
	}
	if e.Op == "and" || e.Op == "or" {
		unknown := false
		for i := range e.Args {
			value, known := staticRuleValue(x, &e.Args[i])
			if !known || !value.known {
				unknown = true
				continue
			}
			if e.Op == "and" && !value.b {
				return booleanValue(false), true
			}
			if e.Op == "or" && value.b {
				return booleanValue(true), true
			}
		}
		if unknown {
			return ruleValue{}, false
		}
		return booleanValue(e.Op == "and"), true
	}
	if e.Op == "lookup" || e.Op == "schedule" {
		key, known := staticRuleValue(x, &e.Args[0])
		if !known {
			return ruleValue{}, false
		}
		if !key.known {
			return key, true
		}
		if e.Ref != "" {
			return x.evaluate(e), true
		}
		for i := 1; i < len(e.Args); i += 2 {
			candidate, _ := staticRuleValue(x, &e.Args[i])
			if valuesEqual(key, candidate) {
				return staticRuleValue(x, &e.Args[i+1])
			}
		}
		return unknownValue("No configured lookup value matches the selected master key"), true
	}
	for i := range e.Args {
		value, known := staticRuleValue(x, &e.Args[i])
		if !known {
			return ruleValue{}, false
		}
		if !value.known {
			return value, true
		}
	}
	return x.evaluate(e), true
}

type publicationChecker struct {
	errors []string
	seen   map[string]bool
	work   int
}

func (p *publicationChecker) add(message string) {
	if !p.seen[message] && len(p.errors) < 100 {
		p.errors = append(p.errors, message)
		p.seen[message] = true
	}
}

func (p *publicationChecker) check(x *ruleEvaluator, e *Expression, path string) {
	if e == nil {
		return
	}
	p.work++
	if p.work > 1000000 {
		p.add("Publication exceeds the supported validation work budget; simplify repeated rules")
		return
	}
	if e.Op == "pending" {
		p.add(path + ": resolve pending policy before publication")
		return
	}
	if e.Op == "ref" && (strings.HasPrefix(e.Ref, "config.") || strings.HasPrefix(e.Ref, "grade.")) {
		value := x.referenceValue(e.Ref)
		if !value.known {
			p.add(path + ": " + value.reason)
		}
		return
	}
	if e.Op == "if" {
		p.check(x, &e.Args[0], path)
		value, known := staticRuleValue(x, &e.Args[0])
		if known && value.known {
			branch := 2
			if value.b {
				branch = 1
			}
			p.check(x, &e.Args[branch], path)
			return
		}
		p.check(x, &e.Args[1], path)
		p.check(x, &e.Args[2], path)
		return
	}
	if e.Op == "lookup" || e.Op == "schedule" {
		p.check(x, &e.Args[0], path)
		key, known := staticRuleValue(x, &e.Args[0])
		if e.Ref != "" {
			if known && key.known {
				value := x.evaluate(e)
				if !value.known {
					p.add(path + ": " + value.reason)
				}
			}
			return
		}
		if known && key.known {
			for i := 1; i < len(e.Args); i += 2 {
				candidate, _ := staticRuleValue(x, &e.Args[i])
				if valuesEqual(key, candidate) {
					p.check(x, &e.Args[i+1], path)
					return
				}
			}
			p.add(path + ": no lookup entry for the selected master value")
			return
		}
	}
	for i := range e.Args {
		p.check(x, &e.Args[i], path)
	}
}

func validateRulePublicationMasters(c Configuration) []string {
	p := publicationChecker{seen: map[string]bool{}}
	data := c.Data
	data.Rules = nil
	raw, _ := json.Marshal(data)
	x := ruleEvaluator{c: c, configMap: map[string]any{}, gradeMap: map[string]any{}, components: map[string]ComponentDefinition{}, groups: map[string]GroupDefinition{}, inputs: map[string]InputDefinition{}, used: map[string]map[string]string{}}
	_ = json.Unmarshal(raw, &x.configMap)
	for _, comp := range c.Data.Rules.Components {
		x.components[comp.Code] = comp
	}
	for _, g := range c.Data.Rules.Groups {
		x.groups[g.Code] = g
	}
	for _, d := range c.Data.Rules.Inputs {
		x.inputs[d.Key] = d
	}
	for _, comp := range c.Data.Rules.Components {
		// Indexed grade/location coverage avoids a rules × grades × locations scan.
		type scopeKey struct{ grade, location string }
		coverage := map[scopeKey]bool{}
		for _, r := range comp.Rules {
			scope, ok := equalityScope(r.When)
			if !ok {
				coverage[scopeKey{}] = true
				continue
			}
			coverage[scopeKey{strings.TrimPrefix(scope["input.gradeId"], "string:"), strings.TrimPrefix(scope["input.locationId"], "string:")}] = true
		}
		if !coverage[scopeKey{}] {
			for _, grade := range c.Data.Grades {
				if coverage[scopeKey{grade.ID, ""}] {
					continue
				}
				for _, loc := range c.Data.Locations {
					p.work++
					if p.work > 1000000 {
						p.add("Publication exceeds the supported validation work budget; simplify repeated scopes")
						return p.errors
					}
					if !coverage[scopeKey{"", loc.ID}] && !coverage[scopeKey{grade.ID, loc.ID}] {
						p.add(comp.Code + " has no rule for grade " + grade.ID + " and location " + loc.ID)
						break
					}
				}
			}
		}
		for _, r := range comp.Rules {
			if r.Status == "pending" {
				p.add("Resolve pending policy " + comp.Code + " before publication")
				continue
			}
			scope, _ := equalityScope(r.When)
			for _, grade := range c.Data.Grades {
				p.work++
				if p.work > 1000000 {
					p.add("Publication exceeds the supported validation work budget; simplify repeated rules")
					return p.errors
				}
				if id := scope["input.gradeId"]; id != "" && id != "string:"+grade.ID {
					continue
				}
				x.in.GradeID = grade.ID
				x.in.LocationID = strings.TrimPrefix(scope["input.locationId"], "string:")
				encoded, _ := json.Marshal(grade)
				x.gradeMap = map[string]any{}
				_ = json.Unmarshal(encoded, &x.gradeMap)
				for _, qualification := range []string{"NONE", "CA", "CMA", "CS"} {
					x.in.Qualification = qualification
					x.steps = 0
					path := comp.Code + " / grade " + grade.ID
					p.check(&x, r.When, path)
					if value, known := staticRuleValue(&x, r.When); known && value.known && !value.b {
						continue
					}
					p.check(&x, r.Eligibility, path)
					if value, known := staticRuleValue(&x, r.Eligibility); known && value.known && !value.b {
						continue
					}
					if r.Status == "not_applicable" {
						continue
					}
					p.check(&x, r.Expression, path)
					p.check(&x, r.Min, path)
					p.check(&x, r.Max, path)
				}
			}
		}
	}
	for _, grade := range c.Data.Grades {
		raw, _ := json.Marshal(grade)
		x.gradeMap = map[string]any{}
		_ = json.Unmarshal(raw, &x.gradeMap)
		x.in.GradeID = grade.ID
		x.in.LocationID = ""
		x.steps = 0
		for _, g := range c.Data.Rules.Groups {
			if g.InGross {
				p.check(&x, g.Target, g.Code+" target")
			}
		}
	}
	return p.errors
}
