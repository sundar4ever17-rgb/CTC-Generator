package engine

import "testing"

func configuredFixture() Configuration {
	c := DemoConfiguration()
	c.ID = "test-config"
	c.Version = 100
	c.Source = "client"
	c.Status = "published"
	c.Data.Grades[0].BasicMin = Str("0")
	c.Data.Grades[0].BasicMax = Str("100000")
	c.Data.Grades[0].ProfessionalAllowance = map[string]string{"CA": "2000", "CMA": "2500", "CS": "3000"}
	c.Data.BasicMethodConfirmed = true
	c.Data.Rounding.Confirmed = true
	c.Data.AnnualizationConfirmed = true
	c.Data.HRAApproved = true
	c.Data.TemplateApproved = true
	inclusive := true
	c.Data.ESI.Ceiling = Str("21000")
	c.Data.ESI.EligibilityBasis = "gross"
	c.Data.ESI.ContributionBasis = "gross"
	c.Data.ESI.CeilingInclusive = &inclusive
	for i := range c.Data.Benefits {
		c.Data.Benefits[i].Kind = "none"
		if c.Data.Benefits[i].Component == "SUPERANNUATION" {
			c.Data.Benefits[i].Period = "monthly"
		} else {
			c.Data.Benefits[i].Period = "annual"
		}
		c.Data.Benefits[i].Notes = "Synthetic test fixture only, not a company policy"
	}
	return c
}
func fixtureInput() Input {
	return Input{CandidateName: "Test Candidate", CandidateReference: "TEST", Designation: "Test role", JoiningDate: "2026-12-01", GradeID: "urd-example", LocationID: "unconfirmed", Qualification: "NONE", Gross: "100000", Basic: "20000", TaxRegime: "new", NPSOptIn: true, CTCYear: 1, Job: map[string]bool{}}
}
func lineOf(r Result, code string) Line {
	for _, l := range r.Lines {
		if l.Code == code {
			return l
		}
	}
	return Line{}
}
func TestExplicitAnnualPercentageBasis(t *testing.T) {
	c := configuredFixture()
	for i := range c.Data.Benefits {
		if c.Data.Benefits[i].Component == "PERFORMANCE" {
			c.Data.Benefits[i] = BenefitRule{Component: "PERFORMANCE", Kind: "percent", Period: "annual", Rate: "0.10", Basis: "gross", BasisPeriod: "annual"}
		}
	}
	r := Calculate(c, fixtureInput())
	if !r.Complete || !r.CanFinalize {
		t.Fatalf("complete configured fixture should finalize: %+v", r.Issues)
	}
	l := lineOf(r, "PERFORMANCE")
	if l.Annual == nil || *l.Annual != "120000.00" {
		t.Fatalf("annual basis was not annualized explicitly: %+v", l)
	}
}
func TestESIThresholdComparisonIsExplicit(t *testing.T) {
	c := configuredFixture()
	c.Data.ESI.Ceiling = Str("100000")
	inclusive := true
	c.Data.ESI.CeilingInclusive = &inclusive
	r := Calculate(c, fixtureInput())
	if got := lineOf(r, "ESI"); got.Monthly == nil || *got.Monthly != "3250.00" {
		t.Fatalf("inclusive threshold: %+v", got)
	}
	inclusive = false
	r = Calculate(c, fixtureInput())
	if got := lineOf(r, "ESI"); got.Monthly == nil || *got.Monthly != "0.00" {
		t.Fatalf("exclusive threshold: %+v", got)
	}
	c.Data.ESI.CeilingInclusive = nil
	r = Calculate(c, fixtureInput())
	if r.CanFinalize || lineOf(r, "ESI").Annual != nil {
		t.Fatal("missing ESI boundary must remain pending")
	}
}
func TestMissingJobParameterIsNotFalse(t *testing.T) {
	c := configuredFixture()
	for i := range c.Data.Benefits {
		if c.Data.Benefits[i].Component == "RELOCATION" {
			c.Data.Benefits[i] = BenefitRule{Component: "RELOCATION", Kind: "fixed", Amount: "5000", Period: "one_time", Conditions: []Condition{{Field: "relocation", Equals: true}}}
		}
	}
	in := fixtureInput()
	r := Calculate(c, in)
	if lineOf(r, "RELOCATION").Status != "pending" {
		t.Fatal("absent parameter must remain unknown")
	}
	in.Job["relocation"] = false
	r = Calculate(c, in)
	if lineOf(r, "RELOCATION").Status != "not_applicable" {
		t.Fatal("explicit false can fail eligibility")
	}
	in.Job["relocation"] = true
	r = Calculate(c, in)
	if got := lineOf(r, "RELOCATION"); got.Annual == nil || *got.Annual != "5000.00" {
		t.Fatal("eligible one-time allowance must count once")
	}
}
func TestConfigRejectsAmbiguousRulesCapsAndCircularBases(t *testing.T) {
	for _, change := range []func(*Configuration){
		func(c *Configuration) { c.Data.Grades[0].ConveyanceCap = Str("100") },
		func(c *Configuration) { c.Data.Benefits = append(c.Data.Benefits, c.Data.Benefits[0]) },
		func(c *Configuration) {
			c.Data.Benefits[0] = BenefitRule{Component: "SUPERANNUATION", Kind: "percent", Rate: "0.1", Basis: "hra", BasisPeriod: "monthly", Period: "monthly"}
		},
		func(c *Configuration) { c.Data.PF.Rate = "1e999999999" },
		func(c *Configuration) {
			c.Data.Benefits[0] = BenefitRule{Component: "SUPERANNUATION", Kind: "fixed", Amount: "500", Period: "monthly"}
		},
	} {
		c := configuredFixture()
		change(&c)
		if len(ValidateConfig(c)) == 0 {
			t.Fatal("invalid configuration accepted")
		}
	}
}
func TestUnknownScheduleYearDoesNotBecomeZero(t *testing.T) {
	c := configuredFixture()
	for i := range c.Data.Benefits {
		if c.Data.Benefits[i].Component == "LTM1" {
			c.Data.Benefits[i] = BenefitRule{Component: "LTM1", Kind: "schedule", Period: "annual", Schedule: []ScheduleItem{{Year: 1, Amount: "10000"}}}
		}
	}
	in := fixtureInput()
	in.CTCYear = 2
	r := Calculate(c, in)
	if lineOf(r, "LTM1").Status != "pending" || r.CanFinalize {
		t.Fatal("unknown schedule year must block finalization")
	}
}
func TestUnboundedNumericInputIsRejected(t *testing.T) {
	for _, value := range []string{"1e999999999", "1000000000000", "NaN", "1.000000000", "-100"} {
		in := fixtureInput()
		in.Gross = value
		r := Calculate(configuredFixture(), in)
		if r.CoreValid || len(r.Issues) == 0 {
			t.Fatalf("invalid numeric value accepted: %s", value)
		}
	}
}
