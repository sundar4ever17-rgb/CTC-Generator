package engine

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var ruleCode = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
var signedRuleDecimal = regexp.MustCompile(`^-?[0-9]{1,12}(\.[0-9]{1,8})?$`)

type ruleValidator struct {
	c          Configuration
	components map[string]ComponentDefinition
	groups     map[string]GroupDefinition
	inputs     map[string]InputDefinition
	errors     []string
	nodes      int
	tooComplex bool
}

func (v *ruleValidator) fail(path, message string) {
	if len(v.errors) < 100 {
		v.errors = append(v.errors, path+": "+message)
	}
}

func (v *ruleValidator) referenceType(ref, path string, target bool) string {
	parts := strings.Split(ref, ".")
	if len(parts) < 2 {
		v.fail(path, "invalid reference "+ref)
		return "invalid"
	}
	switch parts[0] {
	case "component":
		if target {
			v.fail(path, "group targets cannot depend on component results")
		}
		component, ok := v.components[strings.TrimPrefix(ref, "component.")]
		if !ok {
			v.fail(path, "unknown component reference "+ref)
		} else {
			period := ""
			for _, r := range component.Rules {
				if r.Status == "pending" {
					continue
				}
				if period != "" && r.Period != period {
					v.fail(path, "referenced component has ambiguous payment periods: "+ref)
					break
				}
				period = r.Period
			}
		}
		return "number"
	case "target", "total":
		if target {
			v.fail(path, "group targets must depend only on inputs and master parameters")
		}
		g, ok := v.groups[parts[1]]
		if len(parts) != 2 || !ok {
			v.fail(path, "unknown group reference "+ref)
		} else if parts[0] == "target" && !g.InGross {
			v.fail(path, "only Gross groups have targets")
		}
		if parts[0] == "total" {
			for _, comp := range v.components {
				if comp.Group == parts[1] {
					for _, r := range comp.Rules {
						if r.Status != "pending" && r.Period != "monthly" {
							v.fail(path, "total references require monthly components; use explicit component references for annual amounts")
							return "number"
						}
					}
				}
			}
		}
		return "number"
	case "input":
		if len(parts) == 3 && parts[1] == "custom" {
			d, ok := v.inputs[parts[2]]
			if !ok {
				v.fail(path, "unknown custom input "+ref)
				return "invalid"
			}
			switch d.Type {
			case "money", "number":
				return "number"
			case "boolean":
				return "boolean"
			default:
				return "string"
			}
		}
		if len(parts) == 3 && parts[1] == "job" {
			if !jobField(parts[2]) {
				v.fail(path, "unknown job input "+ref)
			}
			return "boolean"
		}
		switch ref {
		case "input.gross", "input.basic", "input.ctcYear":
			return "number"
		case "input.npsOptIn":
			return "boolean"
		case "input.candidateName", "input.candidateReference", "input.designation", "input.department", "input.joiningDate", "input.calculationDate", "input.locationId", "input.gradeId", "input.qualification", "input.taxRegime":
			return "string"
		}
	case "grade":
		switch ref {
		case "grade.basicMin", "grade.basicMax", "grade.conveyance", "grade.conveyanceCap":
			return "number"
		case "grade.id", "grade.name":
			return "string"
		}
	case "config":
		switch ref {
		case "config.pf.basicThreshold", "config.pf.inclusionShare", "config.pf.basisCap", "config.pf.rate", "config.nps.oldRate", "config.nps.newRate", "config.esi.rate", "config.esi.ceiling", "config.gratuityRate", "config.rounding.scale":
			return "number"
		case "config.basicMethodConfirmed", "config.rounding.confirmed", "config.annualizationConfirmed", "config.hraApproved", "config.templateApproved", "config.esi.ceilingInclusive":
			return "boolean"
		case "config.basicMethod", "config.esi.eligibilityBasis", "config.esi.contributionBasis", "config.rounding.mode", "config.annualSpecialGroup":
			return "string"
		}
	}
	v.fail(path, "unknown reference "+ref)
	return "invalid"
}

func (v *ruleValidator) expression(e *Expression, path string, depth int, target bool) string {
	if e == nil {
		v.fail(path, "expression is required")
		return "invalid"
	}
	v.nodes++
	if depth > 32 || v.nodes > 50000 || len(e.Args) > 512 {
		v.tooComplex = true
		v.fail(path, "expression complexity limit exceeded")
		return "invalid"
	}
	if len(e.Value) > 1000 || len(e.Ref) > 200 {
		v.fail(path, "expression text exceeds supported limits")
	}
	count := len(e.Args)
	need := func(n int) {
		if count != n {
			v.fail(path, fmt.Sprintf("%s needs %d arguments", e.Op, n))
		}
	}
	kinds := []string{}
	for i := range e.Args {
		kinds = append(kinds, v.expression(&e.Args[i], fmt.Sprintf("%s.%s[%d]", path, e.Op, i), depth+1, target))
	}
	all := func(kind string) {
		for _, k := range kinds {
			if k != kind && k != "invalid" {
				v.fail(path, e.Op+" requires "+kind+" arguments")
				break
			}
		}
	}
	if e.Op != "ref" && e.Op != "residual" && e.Op != "lookup" && e.Ref != "" {
		v.fail(path, "this operation does not accept ref")
	}
	if e.Op != "number" && e.Op != "string" && e.Op != "boolean" && e.Op != "pending" && e.Value != "" {
		v.fail(path, "this operation does not accept value")
	}
	switch e.Op {
	case "number":
		need(0)
		if !signedRuleDecimal.MatchString(e.Value) {
			v.fail(path, "number must be a bounded decimal string")
		}
		return "number"
	case "string":
		need(0)
		return "string"
	case "boolean":
		need(0)
		if e.Value != "true" && e.Value != "false" {
			v.fail(path, "boolean must be true or false")
		}
		return "boolean"
	case "pending":
		need(0)
		return "number"
	case "ref":
		need(0)
		return v.referenceType(e.Ref, path, target)
	case "residual":
		need(0)
		if target {
			v.fail(path, "target cannot be a residual")
		}
		g, ok := v.groups[e.Ref]
		if !ok || !g.InGross || g.Balancer == "" {
			v.fail(path, "residual must reference a constrained Gross group")
		}
		return "number"
	case "add", "mul", "min", "max":
		if count < 2 {
			v.fail(path, e.Op+" needs at least two arguments")
		}
		all("number")
		return "number"
	case "sub", "div":
		need(2)
		all("number")
		return "number"
	case "and", "or":
		if count < 2 {
			v.fail(path, e.Op+" needs at least two arguments")
		}
		all("boolean")
		return "boolean"
	case "not":
		need(1)
		all("boolean")
		return "boolean"
	case "eq", "ne", "lt", "lte", "gt", "gte":
		need(2)
		if count == 2 && kinds[0] != kinds[1] && kinds[0] != "invalid" && kinds[1] != "invalid" {
			v.fail(path, "comparison arguments must have the same type")
		}
		if count == 2 && e.Op != "eq" && e.Op != "ne" && kinds[0] == "boolean" {
			v.fail(path, "ordered comparison requires numbers or strings")
		}
		return "boolean"
	case "if":
		need(3)
		if count == 3 {
			if kinds[0] != "boolean" && kinds[0] != "invalid" {
				v.fail(path, "if condition must be boolean")
			}
			if kinds[1] != kinds[2] && kinds[1] != "invalid" && kinds[2] != "invalid" {
				v.fail(path, "if branches must have the same type")
			}
			return kinds[1]
		}
		return "invalid"
	case "lookup", "schedule":
		if e.Ref != "" {
			need(1)
			if e.Op != "lookup" || e.Ref != "grade.professionalAllowance" {
				v.fail(path, "unsupported lookup table reference")
			}
			if count == 1 && kinds[0] != "string" {
				v.fail(path, "qualification lookup key must be a string")
			}
			return "number"
		}
		if count < 3 || count%2 != 1 {
			v.fail(path, "lookup needs key followed by key/value pairs")
			return "invalid"
		}
		resultKind := kinds[2]
		seen := map[string]bool{}
		for i := 1; i < count; i += 2 {
			if kinds[i] != kinds[0] {
				v.fail(path, "lookup keys must have the same type")
			}
			if kinds[i+1] != resultKind {
				v.fail(path, "lookup values must have the same type")
			}
			if e.Args[i].Op != "number" && e.Args[i].Op != "string" && e.Args[i].Op != "boolean" {
				v.fail(path, "lookup keys must be constants")
			}
			key := e.Args[i].Op + ":" + e.Args[i].Value
			if e.Args[i].Op == "number" && signedRuleDecimal.MatchString(e.Args[i].Value) {
				key = "number:" + dec(e.Args[i].Value).String()
			}
			if e.Op == "schedule" && e.Args[i].Op == "number" && signedRuleDecimal.MatchString(e.Args[i].Value) {
				year := dec(e.Args[i].Value)
				if !year.IsPositive() || !year.Equal(year.Truncate(0)) || year.GreaterThan(dec("2147483647")) {
					v.fail(path, "schedule years must be positive integers within the supported CTC year range")
				}
			}
			if seen[key] {
				v.fail(path, "duplicate lookup key")
			}
			seen[key] = true
		}
		if e.Op == "schedule" && (kinds[0] != "number" || resultKind != "number") {
			v.fail(path, "schedule needs numeric years and amounts")
		}
		return resultKind
	default:
		v.fail(path, "unsupported operation "+e.Op)
		return "invalid"
	}
}

func expressionDependencies(e *Expression, groups map[string]GroupDefinition, components map[string]ComponentDefinition, out map[string]bool) {
	if e == nil {
		return
	}
	if e.Op == "ref" && strings.HasPrefix(e.Ref, "component.") {
		out[strings.TrimPrefix(e.Ref, "component.")] = true
	}
	if (e.Op == "ref" && strings.HasPrefix(e.Ref, "total.")) || e.Op == "residual" {
		group := strings.TrimPrefix(e.Ref, "total.")
		for _, c := range components {
			if c.Group == group && !(e.Op == "residual" && c.Code == groups[group].Balancer) {
				out[c.Code] = true
			}
		}
	}
	for i := range e.Args {
		expressionDependencies(&e.Args[i], groups, components, out)
	}
}

func componentDependencies(c ComponentDefinition, groups map[string]GroupDefinition, components map[string]ComponentDefinition) []string {
	deps := map[string]bool{}
	for _, r := range c.Rules {
		for _, e := range []*Expression{r.When, r.Eligibility, r.Expression, r.Min, r.Max} {
			expressionDependencies(e, groups, components, deps)
		}
	}
	list := []string{}
	for code := range deps {
		list = append(list, code)
	}
	sort.Strings(list)
	return list
}

func equalityScope(e *Expression) (map[string]string, bool) {
	if e == nil {
		return map[string]string{}, true
	}
	if e.Op == "eq" && len(e.Args) == 2 {
		a, b := e.Args[0], e.Args[1]
		if b.Op == "ref" {
			a, b = b, a
		}
		if a.Op == "ref" && (b.Op == "string" || b.Op == "number" || b.Op == "boolean") {
			value := b.Value
			if b.Op == "number" && signedRuleDecimal.MatchString(value) {
				value = dec(value).String()
			}
			return map[string]string{a.Ref: b.Op + ":" + value}, true
		}
	}
	if e.Op == "and" {
		all := map[string]string{}
		for i := range e.Args {
			part, ok := equalityScope(&e.Args[i])
			if !ok {
				return nil, false
			}
			for k, val := range part {
				if old, exists := all[k]; exists && old != val {
					return nil, false
				}
				all[k] = val
			}
		}
		return all, true
	}
	return nil, false
}

func rulesDisjoint(a, b *Expression) bool {
	x, ok := equalityScope(a)
	if !ok {
		return false
	}
	y, ok := equalityScope(b)
	if !ok {
		return false
	}
	for k, val := range x {
		if other, exists := y[k]; exists && val != other {
			return true
		}
	}
	return false
}

func ValidateRuleConfiguration(c Configuration) []string {
	r := c.Data.Rules
	if r == nil {
		return []string{"Rule configuration is required"}
	}
	if r.Version != 2 {
		return []string{"Unsupported rule configuration version; expected 2"}
	}
	if len(r.Groups) < 1 || len(r.Groups) > 30 || len(r.Components) < 1 || len(r.Components) > 200 || len(r.Inputs) > 100 {
		return []string{"Rules support 1-30 groups, 1-200 components and at most 100 custom inputs"}
	}
	// Existing master parameter types remain guarded. Legacy benefit formulas and
	// component-specific restrictions do not govern v2 component definitions.
	base := c
	base.Data.Rules = nil
	base.Data.Benefits = nil
	base.Data.BasicMethod = "manual"
	base.Data.AnnualSpecialGroup = ""
	v := ruleValidator{c: c, components: map[string]ComponentDefinition{}, groups: map[string]GroupDefinition{}, inputs: map[string]InputDefinition{}, errors: ValidateConfig(base)}
	for _, g := range r.Groups {
		if !ruleCode.MatchString(g.Code) || strings.TrimSpace(g.Name) == "" || len(g.Name) > 200 {
			v.fail("groups", "valid code and name are required")
		}
		if _, ok := v.groups[g.Code]; ok {
			v.fail(g.Code, "duplicate group code")
		}
		v.groups[g.Code] = g
		if g.InGross && (g.Target == nil || g.Balancer == "") {
			v.fail(g.Code, "Gross group requires a target and one balancing component")
		}
		if !g.InGross && (g.Target != nil || g.Balancer != "") {
			v.fail(g.Code, "additional CTC groups cannot define Gross targets or balancing components")
		}
	}
	for _, d := range r.Inputs {
		if !ruleCode.MatchString(d.Key) || strings.TrimSpace(d.Label) == "" || len(d.Label) > 200 {
			v.fail("inputs", "valid input key and label are required")
		}
		if _, ok := v.inputs[d.Key]; ok {
			v.fail(d.Key, "duplicate input key")
		}
		v.inputs[d.Key] = d
		switch d.Type {
		case "money", "number", "boolean", "date":
			if len(d.Options) > 0 {
				v.fail(d.Key, "only enum inputs accept options")
			}
		case "enum":
			if len(d.Options) == 0 || len(d.Options) > 100 {
				v.fail(d.Key, "enum needs 1-100 options")
			}
			seen := map[string]bool{}
			for _, s := range d.Options {
				if s == "" || len(s) > 200 || seen[s] {
					v.fail(d.Key, "enum options must be distinct nonempty strings")
				}
				seen[s] = true
			}
		default:
			v.fail(d.Key, "unsupported custom input type")
		}
	}
	rules := 0
	for _, comp := range r.Components {
		if !ruleCode.MatchString(comp.Code) || strings.TrimSpace(comp.Name) == "" || len(comp.Name) > 200 {
			v.fail("components", "valid component code and name are required")
		}
		if _, ok := v.components[comp.Code]; ok {
			v.fail(comp.Code, "duplicate component code")
		}
		v.components[comp.Code] = comp
		if _, ok := v.groups[comp.Group]; !ok {
			v.fail(comp.Code, "unknown group")
		}
		rules += len(comp.Rules)
		if len(comp.Rules) == 0 || len(comp.Rules) > 1000 {
			v.fail(comp.Code, "component requires 1-1000 scoped rules")
		}
	}
	if rules > 5000 {
		return []string{"At most 5000 calculation rules are supported"}
	}
	grossGroups := 0
	for _, g := range r.Groups {
		if g.InGross {
			grossGroups++
			if v.expression(g.Target, g.Code+" target", 0, true) != "number" {
				v.fail(g.Code, "target must be numeric")
			}
			balancer, ok := v.components[g.Balancer]
			if !ok || balancer.Group != g.Code {
				v.fail(g.Code, "balancer must be a component within the same group")
			}
		}
	}
	if grossGroups == 0 {
		v.fail("groups", "at least one group must participate in Gross")
	}
	for _, comp := range r.Components {
		ids := map[string]bool{}
		for i, rule := range comp.Rules {
			path := fmt.Sprintf("%s rule %d", comp.Code, i+1)
			if len(rule.ID) > 100 || len(rule.Notes) > 10000 {
				v.fail(path, "rule text too long")
			}
			if rule.ID != "" && ids[rule.ID] {
				v.fail(path, "duplicate rule ID")
			}
			ids[rule.ID] = true
			if rule.Status != "" && rule.Status != "calculated" && rule.Status != "not_applicable" && rule.Status != "pending" {
				v.fail(path, "unsupported rule status")
			}
			if rule.LimitsTogether && (rule.Min == nil || rule.Max == nil) {
				v.fail(path, "joint limit validation requires both minimum and maximum")
			}
			if rule.Status != "pending" && rule.Period != "monthly" && rule.Period != "annual" && rule.Period != "one_time" {
				v.fail(path, "explicit payment period is required")
			}
			if v.groups[comp.Group].InGross && rule.Status != "pending" && rule.Period != "monthly" {
				v.fail(path, "components inside Gross must be monthly")
			}
			for label, e := range map[string]*Expression{"when": rule.When, "eligibility": rule.Eligibility} {
				if e != nil && v.expression(e, path+" "+label, 0, false) != "boolean" {
					v.fail(path, label+" must be boolean")
				}
			}
			for label, e := range map[string]*Expression{"minimum": rule.Min, "maximum": rule.Max} {
				if e != nil && v.expression(e, path+" "+label, 0, false) != "number" {
					v.fail(path, label+" must be numeric")
				}
			}
			if rule.Status == "" || rule.Status == "calculated" {
				if v.expression(rule.Expression, path+" amount", 0, false) != "number" {
					v.fail(path, "amount must be numeric")
				}
			} else if rule.Expression != nil {
				v.fail(path, "pending/not-applicable rules must not supply a calculation expression")
				return v.errors
			}
			g := v.groups[comp.Group]
			if g.Balancer == comp.Code {
				if len(comp.Rules) != 1 || rule.When != nil || rule.Eligibility != nil || rule.Expression == nil || rule.Expression.Op != "residual" || rule.Expression.Ref != g.Code || rule.Min != nil || rule.Max != nil || (rule.Status != "" && rule.Status != "calculated") {
					v.fail(path, "balancer requires one unconditional residual rule for its group, without bounds")
				}
			}
			if v.tooComplex {
				return v.errors
			}
			if rule.Expression != nil && containsResidual(rule.Expression) && g.Balancer != comp.Code {
				v.fail(path, "only the designated group balancer may use residual")
			}
			for _, e := range []*Expression{rule.When, rule.Eligibility, rule.Min, rule.Max} {
				if containsResidual(e) {
					v.fail(path, "residual is only permitted as the designated balancing amount")
				}
			}
			for j := 0; j < i; j++ {
				if !rulesDisjoint(comp.Rules[j].When, rule.When) {
					v.fail(path, "scoped rules may overlap; use disjoint equality scopes or one conditional expression")
				}
			}
		}
	}
	state := map[string]int{}
	var visit func(string)
	visit = func(code string) {
		if state[code] == 1 {
			v.fail(code, "circular component dependency")
			return
		}
		if state[code] == 2 {
			return
		}
		state[code] = 1
		for _, dep := range componentDependencies(v.components[code], v.groups, v.components) {
			if _, ok := v.components[dep]; ok {
				visit(dep)
			}
		}
		state[code] = 2
	}
	for _, comp := range r.Components {
		visit(comp.Code)
	}
	return v.errors
}

func containsResidual(e *Expression) bool {
	if e == nil {
		return false
	}
	if e.Op == "residual" {
		return true
	}
	for i := range e.Args {
		if containsResidual(&e.Args[i]) {
			return true
		}
	}
	return false
}

func ValidateRulePublication(c Configuration) []string {
	errs := ValidateRuleConfiguration(c)
	if len(errs) > 0 {
		return errs
	}
	add := func(s string) { errs = append(errs, s) }
	if c.Source != "client" {
		add("Only client policy configurations may be published in production")
	}
	if c.EffectiveFrom == "" {
		add("An effective-from date is required for publication")
	}
	for _, a := range []struct {
		name string
		ok   bool
	}{{"Basic selection", c.Data.BasicMethodConfirmed}, {"Rounding", c.Data.Rounding.Confirmed}, {"Annualization", c.Data.AnnualizationConfirmed}, {"Uncapped HRA treatment", c.Data.HRAApproved}, {"Offer template", c.Data.TemplateApproved}} {
		if !a.ok {
			add(a.name + " must be explicitly approved before publication")
		}
	}
	if strings.TrimSpace(c.Data.OfferTemplate.CompanyName) == "" || strings.TrimSpace(c.Data.OfferTemplate.Address) == "" || strings.TrimSpace(c.Data.OfferTemplate.Signatory) == "" || strings.TrimSpace(c.Data.OfferTemplate.Body) == "" {
		add("The approved offer template requires company name, address, signatory and wording")
	}
	if len(c.Data.Locations) == 0 {
		add("At least one location is required for publication")
	}
	errs = append(errs, validateRulePublicationMasters(c)...)
	if len(errs) > 100 {
		errs = errs[:100]
	}
	return errs
}

func validateCustomInputs(c Configuration, in Input) []Issue {
	var issues []Issue
	defs := map[string]InputDefinition{}
	for _, d := range c.Data.Rules.Inputs {
		defs[d.Key] = d
	}
	for key := range in.Custom {
		if _, ok := defs[key]; !ok {
			issues = append(issues, Issue{Code: "INPUT_INVALID", Field: "custom." + key, Message: "Unknown custom input " + key, Severity: "error"})
		}
	}
	for _, d := range c.Data.Rules.Inputs {
		raw, ok := in.Custom[d.Key]
		if !ok || string(raw) == "null" {
			if d.Required {
				issues = append(issues, Issue{Code: "INPUT_PENDING", Field: "custom." + d.Key, Message: d.Label + " is required", Severity: "pending"})
			}
			continue
		}
		value, err := decodeCustomValue(d, raw)
		if err != "" {
			issues = append(issues, Issue{Code: "INPUT_INVALID", Field: "custom." + d.Key, Message: err, Severity: "error"})
			continue
		}
		if d.Type == "money" && !value.n.Equal(value.n.Round(c.Data.Rounding.Scale)) {
			issues = append(issues, Issue{Code: "INPUT_PRECISION", Field: "custom." + d.Key, Message: d.Label + " exceeds the configured money precision", Severity: "error"})
		}
		if d.Type == "date" {
			if _, err := time.Parse("2006-01-02", value.s); err != nil {
				issues = append(issues, Issue{Code: "INPUT_INVALID", Field: "custom." + d.Key, Message: d.Label + " must be a valid YYYY-MM-DD date", Severity: "error"})
			}
		}
	}
	return issues
}
