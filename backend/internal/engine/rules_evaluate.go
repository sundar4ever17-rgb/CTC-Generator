package engine

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type ruleValue struct {
	kind   string
	n      decimal.Decimal
	s      string
	b      bool
	known  bool
	reason string
	failed bool
}

func numericValue(n decimal.Decimal) ruleValue { return ruleValue{kind: "number", n: n, known: true} }
func stringValue(s string) ruleValue           { return ruleValue{kind: "string", s: s, known: true} }
func booleanValue(b bool) ruleValue            { return ruleValue{kind: "boolean", b: b, known: true} }
func unknownValue(reason string) ruleValue     { return ruleValue{reason: reason} }
func failedValue(reason string) ruleValue      { return ruleValue{reason: reason, failed: true} }

func decodeCustomValue(d InputDefinition, raw json.RawMessage) (ruleValue, string) {
	if len(raw) > 2000 {
		return ruleValue{}, d.Label + " exceeds the supported input size"
	}
	switch d.Type {
	case "money", "number":
		var s string
		if json.Unmarshal(raw, &s) != nil || !signedRuleDecimal.MatchString(s) || (d.Type == "money" && strings.HasPrefix(s, "-")) {
			return ruleValue{}, d.Label + " must be a bounded decimal string"
		}
		return numericValue(dec(s)), ""
	case "boolean":
		var b bool
		if string(raw) == "null" || json.Unmarshal(raw, &b) != nil {
			return ruleValue{}, d.Label + " must explicitly be true or false"
		}
		return booleanValue(b), ""
	default:
		var s string
		if json.Unmarshal(raw, &s) != nil || len(s) > 200 {
			return ruleValue{}, d.Label + " must be a string of at most 200 characters"
		}
		if d.Type == "enum" {
			found := false
			for _, option := range d.Options {
				found = found || s == option
			}
			if !found {
				return ruleValue{}, d.Label + " must match a configured option"
			}
		}
		return stringValue(s), ""
	}
}

type ruleEvaluator struct {
	c          Configuration
	in         Input
	result     Result
	components map[string]ComponentDefinition
	groups     map[string]GroupDefinition
	inputs     map[string]InputDefinition
	values     map[string]ruleValue
	targets    map[string]ruleValue
	state      map[string]int
	indices    map[string]int
	configMap  map[string]any
	gradeMap   map[string]any
	steps      int
	active     string
	used       map[string]map[string]string
}

func (x *ruleEvaluator) issue(code, field, message, severity string) {
	if len(x.result.Issues) < 1000 {
		x.result.Issues = append(x.result.Issues, Issue{Code: code, Field: field, Message: message, Severity: severity})
	}
}
func (x *ruleEvaluator) money(n decimal.Decimal) string {
	return n.Round(x.c.Data.Rounding.Scale).StringFixed(x.c.Data.Rounding.Scale)
}

func mapPath(m map[string]any, path []string) (any, bool) {
	var val any = m
	for _, part := range path {
		obj, ok := val.(map[string]any)
		if !ok {
			return nil, false
		}
		val, ok = obj[part]
		if !ok || val == nil {
			return nil, false
		}
	}
	return val, true
}

func (x *ruleEvaluator) reference(ref string) ruleValue {
	result := x.referenceValue(ref)
	if x.active != "" {
		if x.used[x.active] == nil {
			x.used[x.active] = map[string]string{}
		}
		value := result.reason
		if result.known {
			switch result.kind {
			case "number":
				value = result.n.String()
			case "boolean":
				value = strconv.FormatBool(result.b)
			default:
				value = result.s
			}
		}
		x.used[x.active][ref] = value
	}
	return result
}

func (x *ruleEvaluator) referenceValue(ref string) ruleValue {
	parts := strings.Split(ref, ".")
	if len(parts) < 2 {
		return failedValue("Invalid reference " + ref)
	}
	switch parts[0] {
	case "component":
		return x.component(strings.TrimPrefix(ref, "component."))
	case "target":
		if value, ok := x.targets[parts[1]]; ok {
			return value
		}
		return unknownValue("Unresolved group target " + parts[1])
	case "total":
		sum := decimal.Zero
		for _, c := range x.c.Data.Rules.Components {
			if c.Group == parts[1] {
				v := x.component(c.Code)
				if !v.known {
					return v
				}
				sum = sum.Add(v.n)
			}
		}
		return numericValue(sum)
	case "input":
		if len(parts) == 3 && parts[1] == "custom" {
			d, ok := x.inputs[parts[2]]
			if !ok {
				return failedValue("Unknown custom input " + parts[2])
			}
			raw, ok := x.in.Custom[parts[2]]
			if !ok || string(raw) == "null" {
				return unknownValue("Required input " + d.Label + " was not provided")
			}
			v, err := decodeCustomValue(d, raw)
			if err != "" {
				return failedValue(err)
			}
			return v
		}
		if len(parts) == 3 && parts[1] == "job" {
			v, ok := x.in.Job[parts[2]]
			if !ok {
				return unknownValue("Required job parameter " + parts[2] + " was not provided")
			}
			return booleanValue(v)
		}
		switch ref {
		case "input.gross":
			return numericValue(dec(x.in.Gross))
		case "input.basic":
			if x.in.Basic == "" {
				return unknownValue("Basic input was not provided")
			}
			return numericValue(dec(x.in.Basic))
		case "input.ctcYear":
			return numericValue(decimal.NewFromInt(int64(x.in.CTCYear)))
		case "input.npsOptIn":
			return booleanValue(x.in.NPSOptIn)
		}
		values := map[string]string{"candidateName": x.in.CandidateName, "candidateReference": x.in.CandidateReference, "designation": x.in.Designation, "department": x.in.Department, "joiningDate": x.in.JoiningDate, "calculationDate": x.in.CalculationDate, "locationId": x.in.LocationID, "gradeId": x.in.GradeID, "qualification": x.in.Qualification, "taxRegime": x.in.TaxRegime}
		if v, ok := values[parts[1]]; ok {
			return stringValue(v)
		}
	case "config", "grade":
		m := x.configMap
		if parts[0] == "grade" {
			m = x.gradeMap
		}
		v, ok := mapPath(m, parts[1:])
		if !ok {
			return unknownValue("Required master value " + ref + " is missing")
		}
		switch value := v.(type) {
		case bool:
			return booleanValue(value)
		case float64:
			return numericValue(decimal.NewFromFloat(value))
		case string:
			checker := ruleValidator{components: x.components, groups: x.groups, inputs: x.inputs}
			if checker.referenceType(ref, "", false) == "number" {
				if !signedRuleDecimal.MatchString(value) {
					return failedValue("Invalid numeric master value " + ref)
				}
				return numericValue(dec(value))
			}
			return stringValue(value)
		}
	}
	return failedValue("Unsupported reference " + ref)
}

func valuesEqual(a, b ruleValue) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case "number":
		return a.n.Equal(b.n)
	case "boolean":
		return a.b == b.b
	default:
		return a.s == b.s
	}
}

func (x *ruleEvaluator) evaluate(e *Expression) ruleValue {
	x.steps++
	if x.steps > 200000 {
		return failedValue("Calculation expression budget exceeded")
	}
	if e == nil {
		return unknownValue("Calculation expression is missing")
	}
	switch e.Op {
	case "number":
		return numericValue(dec(e.Value))
	case "string":
		return stringValue(e.Value)
	case "boolean":
		return booleanValue(e.Value == "true")
	case "pending":
		if e.Value != "" {
			return unknownValue(e.Value)
		}
		return unknownValue("Client rule has not been supplied")
	case "ref":
		return x.reference(e.Ref)
	case "residual":
		value := x.reference("target." + e.Ref)
		if !value.known {
			return unknownValue("Group target is unresolved")
		}
		g := x.groups[e.Ref]
		sum := decimal.Zero
		for _, c := range x.c.Data.Rules.Components {
			if c.Group == e.Ref && c.Code != g.Balancer {
				v := x.reference("component." + c.Code)
				if !v.known {
					return unknownValue("Balancing waits for " + c.Name + ": " + v.reason)
				}
				sum = sum.Add(v.n)
			}
		}
		return numericValue(value.n.Sub(sum))
	case "if":
		condition := x.evaluate(&e.Args[0])
		if !condition.known {
			return condition
		}
		if condition.b {
			return x.evaluate(&e.Args[1])
		}
		return x.evaluate(&e.Args[2])
	case "and", "or":
		unknown := ruleValue{}
		for i := range e.Args {
			v := x.evaluate(&e.Args[i])
			if v.failed {
				return v
			}
			if !v.known {
				if unknown.reason == "" {
					unknown = v
				}
				continue
			}
			if e.Op == "and" && !v.b {
				return booleanValue(false)
			}
			if e.Op == "or" && v.b {
				return booleanValue(true)
			}
		}
		if unknown.reason != "" {
			return unknown
		}
		return booleanValue(e.Op == "and")
	case "lookup", "schedule":
		key := x.evaluate(&e.Args[0])
		if !key.known {
			return key
		}
		if e.Ref != "" {
			if e.Ref != "grade.professionalAllowance" {
				return failedValue("Unsupported lookup table")
			}
			v, ok := mapPath(x.gradeMap, []string{"professionalAllowance", key.s})
			if !ok {
				return unknownValue("No configured Professional Allowance slab for " + key.s)
			}
			s, ok := v.(string)
			if !ok {
				return failedValue("Invalid lookup amount")
			}
			return numericValue(dec(s))
		}
		for i := 1; i < len(e.Args); i += 2 {
			candidate := x.evaluate(&e.Args[i])
			if !candidate.known {
				return candidate
			}
			if valuesEqual(key, candidate) {
				return x.evaluate(&e.Args[i+1])
			}
		}
		return unknownValue("No configured " + e.Op + " value matches the selected key")
	}
	args := make([]ruleValue, len(e.Args))
	for i := range e.Args {
		args[i] = x.evaluate(&e.Args[i])
		if !args[i].known {
			return args[i]
		}
	}
	if e.Op == "not" {
		return booleanValue(!args[0].b)
	}
	if e.Op == "eq" {
		return booleanValue(valuesEqual(args[0], args[1]))
	}
	if e.Op == "ne" {
		return booleanValue(!valuesEqual(args[0], args[1]))
	}
	if e.Op == "lt" || e.Op == "lte" || e.Op == "gt" || e.Op == "gte" {
		comparison := 0
		if args[0].kind == "number" {
			comparison = args[0].n.Cmp(args[1].n)
		} else {
			comparison = strings.Compare(args[0].s, args[1].s)
		}
		switch e.Op {
		case "lt":
			return booleanValue(comparison < 0)
		case "lte":
			return booleanValue(comparison <= 0)
		case "gt":
			return booleanValue(comparison > 0)
		default:
			return booleanValue(comparison >= 0)
		}
	}
	result := args[0].n
	for i := 1; i < len(args); i++ {
		n := args[i].n
		switch e.Op {
		case "add":
			result = result.Add(n)
		case "sub":
			result = result.Sub(n)
		case "mul":
			result = result.Mul(n)
		case "div":
			if n.IsZero() {
				return failedValue("Division by zero")
			}
			result = result.DivRound(n, 16)
		case "min":
			if n.LessThan(result) {
				result = n
			}
		case "max":
			if n.GreaterThan(result) {
				result = n
			}
		default:
			return failedValue("Unsupported operation " + e.Op)
		}
		if result.Abs().GreaterThanOrEqual(decimal.New(1, 24)) {
			return failedValue("Intermediate formula amount exceeds supported numeric range")
		}
	}
	return numericValue(result)
}

func (x *ruleEvaluator) component(code string) ruleValue {
	if x.state[code] == 2 {
		return x.values[code]
	}
	if x.state[code] == 1 {
		return failedValue("Circular component dependency at " + code)
	}
	definition, exists := x.components[code]
	if !exists {
		return failedValue("Unknown component " + code)
	}
	x.state[code] = 1
	previous := x.active
	x.active = code
	line := &x.result.Lines[x.indices[code]]
	matchedRule := ""
	defer func() {
		x.result.Trace = append(x.result.Trace, CalculationTrace{Component: code, Rule: matchedRule, Expression: line.Formula, Dependencies: componentDependencies(definition, x.groups, x.components), Values: x.used[code]})
		x.active = previous
		x.state[code] = 2
	}()
	finishUnknown := func(v ruleValue) ruleValue {
		line.Formula = v.reason
		severity := "pending"
		if v.failed {
			severity = "error"
		}
		x.issue("RULE_UNRESOLVED", code, v.reason, severity)
		x.values[code] = v
		return v
	}
	var selected *CalculationRule
	unknownScope := ruleValue{}
	for i := range definition.Rules {
		r := &definition.Rules[i]
		matches := booleanValue(true)
		if r.When != nil {
			matches = x.evaluate(r.When)
		}
		if matches.failed {
			return finishUnknown(matches)
		}
		if !matches.known {
			unknownScope = matches
			continue
		}
		if matches.b {
			if selected != nil {
				return finishUnknown(failedValue("Multiple rules match " + definition.Name))
			}
			selected = r
		}
	}
	if unknownScope.reason != "" {
		return finishUnknown(unknownScope)
	}
	if selected == nil {
		return finishUnknown(unknownValue("No configured rule matches " + definition.Name))
	}
	rule := *selected
	matchedRule = rule.ID
	status := rule.Status
	if status == "" {
		status = "calculated"
	}
	if status == "pending" {
		reason := rule.Notes
		if reason == "" {
			reason = "Client rule has not been supplied"
		}
		return finishUnknown(unknownValue(reason))
	}
	amount := numericValue(decimal.Zero)
	if rule.Eligibility != nil {
		eligible := x.evaluate(rule.Eligibility)
		if !eligible.known {
			return finishUnknown(eligible)
		}
		if !eligible.b {
			status = "not_applicable"
		}
	}
	if status != "not_applicable" {
		amount = x.evaluate(rule.Expression)
		if !amount.known {
			return finishUnknown(amount)
		}
	}
	negative := amount.n.IsNegative()
	amount.n = amount.n.Round(x.c.Data.Rounding.Scale)
	if amount.n.Abs().GreaterThanOrEqual(decimal.New(1, 16)) || (rule.Period == "monthly" && amount.n.Mul(decimal.NewFromInt(12)).Abs().GreaterThanOrEqual(decimal.New(1, 16))) {
		x.issue("AMOUNT_RANGE", code, "Calculated amount exceeds the supported database precision", "error")
	}
	if negative {
		x.issue("NEGATIVE_COMPONENT", code, definition.Name+" is negative; increase the group target or review its rules", "error")
	}
	checkLimits := status != "not_applicable"
	if checkLimits && rule.LimitsTogether {
		for _, bound := range []*Expression{rule.Min, rule.Max} {
			if bound == nil {
				continue
			}
			value := x.evaluate(bound)
			if !value.known {
				severity := "pending"
				if value.failed {
					severity = "error"
				}
				x.issue("LIMIT_UNRESOLVED", code, value.reason, severity)
				checkLimits = false
			}
		}
	}
	if checkLimits {
		for _, bound := range []struct {
			name       string
			expression *Expression
			minimum    bool
		}{{"minimum", rule.Min, true}, {"maximum", rule.Max, false}} {
			if bound.expression == nil {
				continue
			}
			value := x.evaluate(bound.expression)
			if !value.known {
				severity := "pending"
				if value.failed {
					severity = "error"
				}
				x.issue("LIMIT_UNRESOLVED", code, value.reason, severity)
				continue
			}
			if (bound.minimum && amount.n.LessThan(value.n)) || (!bound.minimum && amount.n.GreaterThan(value.n)) {
				x.issue("COMPONENT_LIMIT", code, definition.Name+" violates the configured "+bound.name, "error")
			}
		}
	}
	line.Status = status
	line.Period = rule.Period
	line.Amount = Str(x.money(amount.n))
	if rule.Period == "monthly" {
		line.Monthly = Str(x.money(amount.n))
		line.Annual = Str(x.money(amount.n.Mul(decimal.NewFromInt(12))))
	} else {
		line.Annual = Str(x.money(amount.n))
	}
	if status == "not_applicable" {
		line.Formula = "Not applicable under the configured rule"
		if rule.Notes != "" {
			line.Formula += " · " + rule.Notes
		}
	} else {
		line.Formula = FormatExpression(rule.Expression)
	}
	x.values[code] = amount
	return amount
}

// FormatExpression is a display-only description; it is never parsed or executed.
func FormatExpression(e *Expression) string {
	if e == nil {
		return "Unresolved rule"
	}
	switch e.Op {
	case "number", "boolean":
		return e.Value
	case "string":
		return strconv.Quote(e.Value)
	case "ref":
		return e.Ref
	case "pending":
		return "Pending: " + e.Value
	case "residual":
		return "target." + e.Ref + " - sum(other applicable components in " + e.Ref + ")"
	}
	args := []string{}
	if e.Ref != "" {
		args = append(args, e.Ref)
	}
	for i := range e.Args {
		args = append(args, FormatExpression(&e.Args[i]))
	}
	return e.Op + "(" + strings.Join(args, ", ") + ")"
}

func CalculateRules(c Configuration, in Input) Result {
	x := ruleEvaluator{c: c, in: in, result: Result{ConfigurationID: c.ID, ConfigurationVersion: c.Version, EngineVersion: RuleEngineVersion, Lines: []Line{}, Issues: []Issue{}, Groups: []GroupResult{}, Trace: []CalculationTrace{}}, components: map[string]ComponentDefinition{}, groups: map[string]GroupDefinition{}, inputs: map[string]InputDefinition{}, values: map[string]ruleValue{}, targets: map[string]ruleValue{}, state: map[string]int{}, indices: map[string]int{}, used: map[string]map[string]string{}}
	if errors := ValidateRuleConfiguration(c); len(errors) > 0 {
		for _, err := range errors {
			x.issue("CONFIG_INVALID", "configuration", err, "error")
		}
		return x.result
	}
	for _, g := range c.Data.Rules.Groups {
		x.groups[g.Code] = g
	}
	for _, d := range c.Data.Rules.Inputs {
		x.inputs[d.Key] = d
	}
	for _, comp := range c.Data.Rules.Components {
		x.components[comp.Code] = comp
		x.indices[comp.Code] = len(x.result.Lines)
		x.result.Lines = append(x.result.Lines, Line{Code: comp.Code, Name: comp.Name, Group: comp.Group, Status: "pending", Formula: "Required configuration is missing"})
	}
	x.result.Issues = append(x.result.Issues, ValidateInput(in)...)
	x.result.Issues = append(x.result.Issues, validateCustomInputs(c, in)...)
	if !validDecimal(in.Gross) || !dec(in.Gross).IsPositive() {
		x.issue("GROSS_INVALID", "gross", "Monthly Gross must be a positive decimal below 1 trillion", "error")
		return x.result
	}
	gross := dec(in.Gross)
	x.result.Totals.Gross = x.money(gross)
	if !gross.Equal(gross.Round(c.Data.Rounding.Scale)) {
		x.issue("INPUT_PRECISION", "gross", "Input money exceeds the selected rounding precision", "error")
	}
	if in.Basic != "" && (!validDecimal(in.Basic) || !dec(in.Basic).Equal(dec(in.Basic).Round(c.Data.Rounding.Scale))) {
		x.issue("INPUT_PRECISION", "basic", "Basic must be a nonnegative decimal within the configured precision", "error")
	}
	if in.TaxRegime != "old" && in.TaxRegime != "new" {
		x.issue("REGIME_INVALID", "taxRegime", "Tax regime must be new or old", "error")
	}
	if in.Qualification != "NONE" && in.Qualification != "CA" && in.Qualification != "CMA" && in.Qualification != "CS" {
		x.issue("QUALIFICATION_INVALID", "qualification", "Choose NONE, CA, CMA or CS", "error")
	}
	if in.CTCYear < 1 {
		x.issue("YEAR_INVALID", "ctcYear", "CTC attribution year must be positive", "error")
	}
	if in.CalculationDate != "" {
		if _, err := time.Parse("2006-01-02", in.CalculationDate); err == nil && ((c.EffectiveFrom != "" && in.CalculationDate < c.EffectiveFrom) || (c.EffectiveTo != "" && in.CalculationDate > c.EffectiveTo)) {
			x.issue("CONFIG_NOT_EFFECTIVE", "calculationDate", "Selected configuration is not effective on the calculation date", "error")
		}
	}
	var grade *Grade
	for i := range c.Data.Grades {
		if c.Data.Grades[i].ID == in.GradeID {
			grade = &c.Data.Grades[i]
		}
	}
	if grade == nil {
		x.issue("GRADE_INVALID", "gradeId", "Select a configured grade", "error")
		return x.result
	}
	locationFound := false
	for _, l := range c.Data.Locations {
		locationFound = locationFound || l.ID == in.LocationID
	}
	if !locationFound {
		x.issue("LOCATION_INVALID", "locationId", "Select a configured location", "error")
	}
	data := c.Data
	data.Rules = nil
	encoded, _ := json.Marshal(data)
	_ = json.Unmarshal(encoded, &x.configMap)
	encoded, _ = json.Marshal(grade)
	_ = json.Unmarshal(encoded, &x.gradeMap)
	for _, a := range []struct {
		code, field, message string
		confirmed            bool
	}{{"BASIC_METHOD_PENDING", "configuration", "Basic selection method requires confirmation", c.Data.BasicMethodConfirmed}, {"ROUNDING_PENDING", "configuration", "Rounding convention requires confirmation", c.Data.Rounding.Confirmed}, {"ANNUALIZATION_PENDING", "configuration", "Annualization convention requires confirmation", c.Data.AnnualizationConfirmed}, {"HRA_APPROVAL_PENDING", "configuration", "Finance/Tax HRA treatment requires confirmation", c.Data.HRAApproved}} {
		if !a.confirmed {
			x.issue(a.code, a.field, a.message, "pending")
		}
	}
	targetSum := decimal.Zero
	targetsKnown := true
	for _, g := range c.Data.Rules.Groups {
		if !g.InGross {
			continue
		}
		value := x.evaluate(g.Target)
		x.targets[g.Code] = value
		if !value.known {
			targetsKnown = false
			severity := "pending"
			if value.failed {
				severity = "error"
			}
			x.issue("TARGET_UNRESOLVED", g.Code, value.reason, severity)
			continue
		}
		if value.n.IsNegative() {
			x.issue("TARGET_INVALID", g.Code, "Group target cannot be negative", "error")
		}
		if !value.n.Equal(value.n.Round(c.Data.Rounding.Scale)) {
			x.issue("GROSS_UNSPLITTABLE", g.Code, "Group target cannot be represented exactly at configured precision", "error")
		}
		targetSum = targetSum.Add(value.n)
	}
	if targetsKnown && !targetSum.Equal(gross) {
		x.issue("TARGET_TOTAL", "gross", "Configured Gross group targets must sum exactly to Monthly Gross", "error")
	}
	for _, comp := range c.Data.Rules.Components {
		x.component(comp.Code)
	}
	knownAnnual := decimal.Zero
	allKnown := true
	coreKnown := targetsKnown
	grossActual := decimal.Zero
	for _, g := range c.Data.Rules.Groups {
		monthly, annual := decimal.Zero, decimal.Zero
		known := true
		for _, line := range x.result.Lines {
			if line.Group != g.Code {
				continue
			}
			if line.Annual == nil {
				known = false
				allKnown = false
				continue
			}
			annual = annual.Add(dec(*line.Annual))
			knownAnnual = knownAnnual.Add(dec(*line.Annual))
			if line.Monthly != nil {
				monthly = monthly.Add(dec(*line.Monthly))
			}
		}
		gr := GroupResult{Code: g.Code, Name: g.Name, InGross: g.InGross, Valid: known}
		if known {
			gr.Monthly = Str(x.money(monthly))
			gr.Annual = Str(x.money(annual))
		}
		if g.InGross {
			target := x.targets[g.Code]
			if target.known {
				gr.Target = Str(x.money(target.n))
			}
			gr.Valid = known && target.known && monthly.Equal(target.n)
			if known && target.known {
				gr.Difference = Str(x.money(monthly.Sub(target.n)))
				if !monthly.Equal(target.n) {
					x.issue("PARITY_FAILED", g.Code, "Actual group total does not equal its configured target", "error")
				}
			}
			coreKnown = coreKnown && gr.Valid
			grossActual = grossActual.Add(monthly)
		}
		x.result.Groups = append(x.result.Groups, gr)
		switch g.Code {
		case "inclusion":
			x.result.Totals.Inclusion = gr.Monthly
		case "exclusion":
			x.result.Totals.Exclusion = gr.Monthly
		case "statutory":
			x.result.Totals.StatutoryAnnual = gr.Annual
		case "policy":
			x.result.Totals.PolicyAnnual = gr.Annual
		case "variable":
			x.result.Totals.VariableAnnual = gr.Annual
		}
	}
	if x.result.Totals.Inclusion != nil && x.result.Totals.Exclusion != nil {
		x.result.Totals.Difference = Str(x.money(dec(*x.result.Totals.Inclusion).Sub(dec(*x.result.Totals.Exclusion))))
	}
	x.result.Totals.KnownAnnualTotal = x.money(knownAnnual)
	hasError, hasPending := false, false
	for _, issue := range x.result.Issues {
		hasError = hasError || issue.Severity == "error"
		hasPending = hasPending || issue.Severity == "pending"
	}
	x.result.CoreValid = !hasError && coreKnown && grossActual.Equal(gross)
	x.result.Complete = x.result.CoreValid && allKnown && !hasPending
	if x.result.Complete {
		x.result.Totals.AnnualCTC = Str(x.money(knownAnnual))
	}
	x.result.CanFinalize = x.result.Complete && c.Source == "client" && c.Status == "published" && c.Data.TemplateApproved && strings.TrimSpace(c.Data.OfferTemplate.CompanyName) != "" && strings.TrimSpace(c.Data.OfferTemplate.Body) != "" && strings.TrimSpace(c.Data.OfferTemplate.Signatory) != ""
	if c.Source != "client" {
		x.issue("DEMO_CONFIGURATION", "configuration", "Illustrative configuration: outputs are demonstration drafts only", "warning")
	}
	if c.Status != "published" {
		x.issue("DRAFT_CONFIGURATION", "configuration", "Publish this configuration before final documents can be generated", "warning")
	}
	if !c.Data.TemplateApproved {
		x.issue("TEMPLATE_PENDING", "configuration", "Offer wording and company fields require approval", "warning")
	}
	// Stable trace order follows the component catalog, independently of lazy
	// dependency evaluation and conditional branches.
	sort.SliceStable(x.result.Trace, func(i, j int) bool {
		return x.indices[x.result.Trace[i].Component] < x.indices[x.result.Trace[j].Component]
	})
	return x.result
}
