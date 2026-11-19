package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func v2Fixture(c Configuration) Configuration {
	out := ConvertConfiguration(c)
	out.ID = c.ID
	out.Version = c.Version
	out.Status = c.Status
	return out
}
func v2Component(c *Configuration, code string) *ComponentDefinition {
	for i := range c.Data.Rules.Components {
		if c.Data.Rules.Components[i].Code == code {
			return &c.Data.Rules.Components[i]
		}
	}
	return nil
}

func TestRuleConversionPreservesLegacyCalculations(t *testing.T) {
	for _, base := range []Configuration{DemoConfiguration(), configuredFixture()} {
		for _, gross := range []string{"10000", "42000", "80000", "100000", "100000.01", "120000"} {
			for _, basic := range []string{"1000", "20000", "25000", "25001", "30000"} {
				for _, qualification := range []string{"NONE", "CA", "CMA", "CS"} {
					for _, regime := range []string{"old", "new"} {
						for _, opt := range []bool{false, true} {
							in := fixtureInput()
							in.Gross = gross
							in.Basic = basic
							in.Qualification = qualification
							in.TaxRegime = regime
							in.NPSOptIn = opt
							old, new := Calculate(base, in), Calculate(v2Fixture(base), in)
							if !reflect.DeepEqual(old.Totals, new.Totals) || old.CoreValid != new.CoreValid || old.Complete != new.Complete || old.CanFinalize != new.CanFinalize {
								t.Fatalf("conversion mismatch gross=%s basic=%s qual=%s regime=%s opt=%v\nlegacy=%+v valid=%v complete=%v\nv2=%+v valid=%v complete=%v issues=%+v", gross, basic, qualification, regime, opt, old.Totals, old.CoreValid, old.Complete, new.Totals, new.CoreValid, new.Complete, new.Issues)
							}
							for _, line := range old.Lines {
								n := lineOf(new, line.Code)
								if !reflect.DeepEqual(line.Monthly, n.Monthly) || !reflect.DeepEqual(line.Annual, n.Annual) || !reflect.DeepEqual(line.Amount, n.Amount) || line.Status != n.Status || line.Group != n.Group || line.Period != n.Period {
									t.Fatalf("component mismatch %s: old=%+v new=%+v", line.Code, line, n)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestRuleConversionPreservesBenefitsRoundingAndUnknowns(t *testing.T) {
	c := configuredFixture()
	c.Data.Grades[0].BasicMin = nil
	c.Data.Grades[0].ConveyanceCap = nil
	for i, b := range c.Data.Benefits {
		switch b.Component {
		case "SUPERANNUATION":
			c.Data.Benefits[i] = BenefitRule{Component: b.Component, Kind: "percent", Rate: "0.032525", Basis: "basic", BasisPeriod: "monthly", Period: "monthly"}
		case "ANNUAL_SPECIAL":
			c.Data.Benefits[i] = BenefitRule{Component: b.Component, Kind: "fixed", Amount: "16000", Period: "annual"}
		case "PERFORMANCE":
			c.Data.Benefits[i] = BenefitRule{Component: b.Component, Kind: "percent", Rate: "0.1", Basis: "hra", BasisPeriod: "annual", Period: "annual", Cap: "40000"}
		case "RELOCATION":
			c.Data.Benefits[i] = BenefitRule{Component: b.Component, Kind: "fixed", Amount: "7000", Period: "one_time", Conditions: []Condition{{Field: "relocation", Equals: true}, {Field: "fieldService", Equals: true}}}
		case "LTM1":
			c.Data.Benefits[i] = BenefitRule{Component: b.Component, Kind: "schedule", Period: "annual", Schedule: []ScheduleItem{{Year: 1, Amount: "25000"}, {Year: 2, Amount: "35000"}}}
		}
	}
	for _, annualConfirmed := range []bool{true, false} {
		c.Data.AnnualizationConfirmed = annualConfirmed
		for year := 1; year <= 3; year++ {
			for _, job := range []map[string]bool{{}, {"relocation": false}, {"relocation": true}, {"relocation": true, "fieldService": true}} {
				in := fixtureInput()
				in.Basic = "20000.13"
				in.CTCYear = year
				in.Job = job
				old, new := Calculate(c, in), Calculate(v2Fixture(c), in)
				if !reflect.DeepEqual(old.Totals, new.Totals) || old.CoreValid != new.CoreValid || old.Complete != new.Complete {
					t.Fatalf("benefit conversion mismatch old=%+v new=%+v issues=%+v", old.Totals, new.Totals, new.Issues)
				}
				for _, line := range old.Lines {
					n := lineOf(new, line.Code)
					if !reflect.DeepEqual(line.Amount, n.Amount) || line.Status != n.Status {
						t.Fatalf("benefit mismatch %s old=%+v new=%+v", line.Code, line, n)
					}
				}
			}
		}
	}
}

func TestDynamicComponentsTargetsConditionsAndDerivedBasic(t *testing.T) {
	c := v2Fixture(configuredFixture())
	c.Data.Rules.Inputs = []InputDefinition{{Key: "skillEligible", Label: "Skill eligible", Type: "boolean"}, {Key: "skillAmount", Label: "Skill amount", Type: "money"}}
	c.Data.Rules.Components = append(c.Data.Rules.Components, ComponentDefinition{Code: "SKILL", Name: "Skill allowance", Group: "inclusion", Rules: []CalculationRule{{ID: "skill", Period: "monthly", Eligibility: expressionPointer(ruleRef("input.custom.skillEligible")), Expression: expressionPointer(ruleRef("input.custom.skillAmount"))}}})
	in := fixtureInput()
	in.Custom = map[string]json.RawMessage{"skillEligible": json.RawMessage("true"), "skillAmount": json.RawMessage(`"3000"`)}
	r := Calculate(c, in)
	if !r.CanFinalize || *lineOf(r, "SPECIAL").Amount != "27000.00" || *r.Totals.Inclusion != "50000.00" {
		t.Fatalf("dynamic inclusion failed: %+v", r)
	}
	in.Custom["skillEligible"] = json.RawMessage("false")
	delete(in.Custom, "skillAmount")
	r = Calculate(c, in)
	if !r.CanFinalize || lineOf(r, "SKILL").Status != "not_applicable" {
		t.Fatalf("irrelevant missing input must not block: %+v", r.Issues)
	}
	v2Component(&c, "BASIC").Rules[0].Expression = expressionPointer(ruleOp("mul", ruleRef("input.gross"), ruleNumber("0.2")))
	in.Basic = ""
	r = Calculate(c, in)
	if !r.CanFinalize || *lineOf(r, "BASIC").Amount != "20000.00" || *lineOf(r, "NPS").Amount != "2800.00" {
		t.Fatalf("derived Basic failed: %+v", r.Issues)
	}
	c.Data.Rules.Groups[0].Target = expressionPointer(ruleOp("mul", ruleRef("input.gross"), ruleNumber("0.6")))
	c.Data.Rules.Groups[1].Target = expressionPointer(ruleOp("mul", ruleRef("input.gross"), ruleNumber("0.4")))
	r = Calculate(c, in)
	if !r.CoreValid || *r.Totals.Inclusion != "60000.00" || *r.Totals.Exclusion != "40000.00" {
		t.Fatalf("configured ratio failed: %+v", r.Issues)
	}
	for _, g := range r.Groups {
		if g.InGross && (!g.Valid || *g.Difference != "0.00") {
			t.Fatalf("ratio-aware target validation failed: %+v", g)
		}
	}
}

func TestRuleEngineRejectsUnsafeOrAmbiguousConfigurations(t *testing.T) {
	for _, change := range []func(*Configuration){
		func(c *Configuration) { c.Data.Rules.Version = 99 },
		func(c *Configuration) {
			v2Component(c, "BASIC").Rules[0].Expression = expressionPointer(ruleRef("component.SPECIAL"))
		},
		func(c *Configuration) {
			v2Component(c, "BASIC").Rules[0].Eligibility = expressionPointer(ruleOp("gt", ruleRef("total.inclusion"), ruleNumber("0")))
		},
		func(c *Configuration) {
			v2Component(c, "NPS").Rules[0].Expression = expressionPointer(ruleRef("component.MISSING"))
		},
		func(c *Configuration) {
			v2Component(c, "NPS").Rules[0].Expression = expressionPointer(ruleOp("exec", ruleString("anything")))
		},
		func(c *Configuration) {
			v2Component(c, "NPS").Rules[0].Expression = expressionPointer(ruleOp("add", ruleString("3"), ruleNumber("2")))
		},
		func(c *Configuration) { v2Component(c, "NPS").Rules[0].Period = "annual" },
		func(c *Configuration) {
			v2Component(c, "NPS").Rules[0].Expression = expressionPointer(ruleNumber("1e1000"))
		},
		func(c *Configuration) { c.Data.Rules.Groups[0].Target = expressionPointer(ruleRef("component.BASIC")) },
		func(c *Configuration) { n := v2Component(c, "NPS"); n.Rules = append(n.Rules, n.Rules[0]) },
	} {
		c := v2Fixture(configuredFixture())
		change(&c)
		if len(ValidateConfig(c)) == 0 {
			t.Fatal("unsafe configuration accepted")
		}
		if Calculate(c, fixtureInput()).CoreValid {
			t.Fatal("invalid configuration calculated")
		}
	}
	c := v2Fixture(configuredFixture())
	v2Component(&c, "NPS").Rules[0].Expression = expressionPointer(ruleOp("div", ruleNumber("1"), ruleNumber("0")))
	r := Calculate(c, fixtureInput())
	if r.CoreValid {
		t.Fatal("division by zero accepted")
	}
}

func TestRuleConverterDoesNotMutatePublishedConfiguration(t *testing.T) {
	c := configuredFixture()
	before, _ := json.Marshal(c)
	converted := ConvertConfiguration(c)
	converted.Data.Grades[0].Name = "changed"
	after, _ := json.Marshal(c)
	if string(before) != string(after) || converted.ID != "" || converted.Version != 0 || converted.Status != "draft" {
		t.Fatal("converter mutated legacy configuration or retained persisted identity")
	}
}

func TestRulePublicationChecksReferencedMastersAndReachablePending(t *testing.T) {
	c := publicationFixture()
	for i, b := range c.Data.Benefits {
		if b.Component == "PERFORMANCE" {
			c.Data.Benefits[i] = BenefitRule{Component: b.Component, Kind: "percent", Rate: "0.1", Basis: "gross", BasisPeriod: "annual", Period: "annual"}
		}
	}
	v := v2Fixture(c)
	if errs := ValidatePublication(v); len(errs) > 0 {
		t.Fatalf("approved annual formula must publish: %v", errs)
	}
	for _, change := range []func(*Configuration){func(c *Configuration) { c.Data.ESI.Ceiling = nil }, func(c *Configuration) { c.Data.ESI.EligibilityBasis = "" }, func(c *Configuration) { c.Data.Grades[0].BasicMin = nil }, func(c *Configuration) { delete(c.Data.Grades[0].ProfessionalAllowance, "CA") }} {
		copy := v2Fixture(c)
		change(&copy)
		if len(ValidatePublication(copy)) == 0 {
			t.Fatal("incomplete referenced master allowed to publish")
		}
	}
	for _, code := range []string{"ESI", "PROFESSIONAL"} {
		v2Component(&v, code).Rules = []CalculationRule{{Period: "monthly", Status: "not_applicable"}}
	}
	v.Data.ESI.Ceiling = nil
	v.Data.Grades[0].ProfessionalAllowance = map[string]string{}
	if errs := ValidatePublication(v); len(errs) > 0 {
		t.Fatalf("unused old parameters must not block generic rule publication: %v", errs)
	}
}

func TestNumericScopeAliasesAreNotDisjoint(t *testing.T) {
	c := v2Fixture(configuredFixture())
	component := v2Component(&c, "PERFORMANCE")
	component.Rules = []CalculationRule{{ID: "a", Period: "annual", When: expressionPointer(ruleOp("eq", ruleRef("input.ctcYear"), ruleNumber("1"))), Expression: expressionPointer(ruleNumber("1"))}, {ID: "b", Period: "annual", When: expressionPointer(ruleOp("eq", ruleRef("input.ctcYear"), ruleNumber("1.0"))), Expression: expressionPointer(ruleNumber("2"))}}
	if errs := ValidateConfig(c); len(errs) == 0 || !strings.Contains(strings.Join(errs, " "), "overlap") {
		t.Fatalf("numerically identical rule scopes must overlap: %v", errs)
	}
}

func TestConvertedIncompleteRangeAndESIRemainPending(t *testing.T) {
	for _, change := range []func(*Configuration){
		func(c *Configuration) { c.Data.Grades[0].BasicMin = nil; c.Data.Grades[0].BasicMax = Str("10000") },
		func(c *Configuration) { c.Data.Grades[0].BasicMax = nil; c.Data.Grades[0].BasicMin = Str("30000") },
		func(c *Configuration) { c.Data.ESI.ContributionBasis = "" },
		func(c *Configuration) { c.Data.ESI.EligibilityBasis = "" },
		func(c *Configuration) { c.Data.ESI.CeilingInclusive = nil },
	} {
		c := configuredFixture()
		change(&c)
		old, new := Calculate(c, fixtureInput()), Calculate(v2Fixture(c), fixtureInput())
		if !reflect.DeepEqual(old.Totals, new.Totals) || old.CoreValid != new.CoreValid || old.Complete != new.Complete {
			t.Fatalf("partial master mismatch old=%+v new=%+v issues=%+v", old.Totals, new.Totals, new.Issues)
		}
		for _, line := range old.Lines {
			n := lineOf(new, line.Code)
			if !reflect.DeepEqual(line.Amount, n.Amount) || line.Status != n.Status || line.Period != n.Period {
				t.Fatalf("partial master line mismatch old=%+v new=%+v", line, n)
			}
		}
	}
}

func TestRuleDependenciesUseRoundedValuesAndTraceActualInputs(t *testing.T) {
	c := v2Fixture(configuredFixture())
	c.Data.Rules.Components = append(c.Data.Rules.Components,
		ComponentDefinition{Code: "ROUND_A", Name: "Round A", Group: "inclusion", Rules: []CalculationRule{{Period: "monthly", Expression: expressionPointer(ruleNumber("0.005"))}}},
		ComponentDefinition{Code: "ROUND_B", Name: "Round B", Group: "inclusion", Rules: []CalculationRule{{Period: "monthly", Expression: expressionPointer(ruleOp("mul", ruleRef("component.ROUND_A"), ruleNumber("2")))}}})
	r := Calculate(c, fixtureInput())
	if !r.CoreValid || *lineOf(r, "ROUND_A").Amount != "0.01" || *lineOf(r, "ROUND_B").Amount != "0.02" || *lineOf(r, "SPECIAL").Amount != "29999.97" {
		t.Fatalf("rounded dependency or residual mismatch: %+v", r)
	}
	found := false
	for _, trace := range r.Trace {
		if trace.Component == "ROUND_B" {
			found = true
			if trace.Values["component.ROUND_A"] != "0.01" {
				t.Fatalf("trace omits rounded dependency: %+v", trace)
			}
		}
	}
	if !found {
		t.Fatal("missing component trace")
	}
	v2Component(&c, "ROUND_B").Rules[0].Expression = expressionPointer(ruleNumber("-0.001"))
	if Calculate(c, fixtureInput()).CoreValid {
		t.Fatal("negative raw amount hidden by rounding")
	}
}

func TestExpressionComplexityIsRejectedBeforeEvaluation(t *testing.T) {
	c := v2Fixture(configuredFixture())
	expr := ruleNumber("1")
	for i := 0; i < 80; i++ {
		expr = ruleOp("add", expr, ruleNumber("1"))
	}
	v2Component(&c, "NPS").Rules[0].Expression = &expr
	if len(ValidateConfig(c)) == 0 {
		t.Fatal("excessive nesting accepted")
	}
	if Calculate(c, fixtureInput()).CoreValid {
		t.Fatal("excessive formula evaluated")
	}
}

func TestRuleScheduleYearsRequireSupportedPositiveIntegers(t *testing.T) {
	for _, year := range []string{"0", "-1", "1.5", "2147483648"} {
		c := v2Fixture(configuredFixture())
		v2Component(&c, "LTM1").Rules = []CalculationRule{{Period: "annual", Expression: expressionPointer(ruleOp("schedule", ruleRef("input.ctcYear"), ruleNumber(year), ruleNumber("100")))}}
		if len(ValidateConfig(c)) == 0 {
			t.Fatalf("invalid schedule year %s accepted", year)
		}
	}
	c := v2Fixture(configuredFixture())
	v2Component(&c, "LTM1").Rules = []CalculationRule{{Period: "annual", Expression: expressionPointer(ruleOp("schedule", ruleRef("input.ctcYear"), ruleNumber("1"), ruleNumber("100"), ruleNumber("2147483647"), ruleNumber("200")))}}
	if errors := ValidateConfig(c); len(errors) > 0 {
		t.Fatalf("valid schedule boundaries rejected: %v", errors)
	}
}
