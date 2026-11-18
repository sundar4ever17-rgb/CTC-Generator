package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputRejectsUnknownFieldsAndNullElections(t *testing.T) {
	for _, payload := range []string{
		`{"npsOptIn":true,"grosss":"100000"}`,
		`{"npsOptIn":true,"job":{"relocation":null}}`,
		`{"npsOptIn":false,"job":{"unrecognized":false}}`,
	} {
		var in Input
		if err := json.Unmarshal([]byte(payload), &in); err == nil {
			t.Fatalf("unsafe or ambiguous input accepted: %s", payload)
		}
	}
	var in Input
	if err := json.Unmarshal([]byte(`{"npsOptIn":false,"job":{"relocation":false}}`), &in); err != nil {
		t.Fatalf("explicit negative elections must be valid: %v", err)
	}
	if value, exists := in.Job["relocation"]; !exists || value {
		t.Fatal("explicit false election was not preserved")
	}
}

func TestPolicyConditionRequiresExplicitBoolean(t *testing.T) {
	for _, payload := range []string{`{"field":"relocation"}`, `{"field":"relocation","equals":null}`, `{"field":"relocation","equals":true,"eqauls":false}`} {
		var condition Condition
		if json.Unmarshal([]byte(payload), &condition) == nil {
			t.Fatalf("ambiguous eligibility condition accepted: %s", payload)
		}
	}
}

func TestCalculationDateUsesExplicitInclusiveWindow(t *testing.T) {
	c := configuredFixture()
	c.EffectiveFrom, c.EffectiveTo = "2026-01-01", "2026-12-31"
	for _, tc := range []struct {
		date  string
		valid bool
	}{{"2025-12-31", false}, {"2026-01-01", true}, {"2026-12-31", true}, {"2027-01-01", false}, {"2026-02-30", false}, {"2026-1-01", false}} {
		in := fixtureInput()
		in.CalculationDate = tc.date
		// A different joining date never changes policy selection implicitly.
		in.JoiningDate = "2028-01-01"
		r := Calculate(c, in)
		if r.CanFinalize != tc.valid {
			t.Fatalf("date %s: expected valid=%t; issues=%+v", tc.date, tc.valid, r.Issues)
		}
	}
}

func TestProfileValidationBlocksMalformedDateAndOversizedData(t *testing.T) {
	for _, change := range []func(*Input){
		func(in *Input) { in.JoiningDate = "2026-02-30" },
		func(in *Input) { in.CandidateName = strings.Repeat("x", 201) },
		func(in *Input) { in.Notes = "bad\x00notes" },
		func(in *Input) { in.CTCYear = 2147483648 },
		func(in *Input) { in.Job["typo"] = true },
	} {
		in := fixtureInput()
		change(&in)
		if r := Calculate(configuredFixture(), in); r.CoreValid || r.CanFinalize {
			t.Fatal("malformed profile must block saving/finalization")
		}
	}
}

func publicationFixture() Configuration {
	c := configuredFixture()
	c.EffectiveFrom = "2026-01-01"
	c.Data.AnnualSpecialGroup = "statutory"
	c.Data.OfferTemplate.Address = "Synthetic test address"
	return c
}

func TestProductionPublicationRequiresCompleteApprovedPolicies(t *testing.T) {
	if errs := ValidatePublication(publicationFixture()); len(errs) != 0 {
		t.Fatalf("complete explicit fixture rejected: %v", errs)
	}
	for _, tc := range []struct {
		name   string
		change func(*Configuration)
	}{
		{"illustrative_source", func(c *Configuration) { c.Source = "urd-demo" }},
		{"effective_date", func(c *Configuration) { c.EffectiveFrom = "" }},
		{"unapproved_rounding", func(c *Configuration) { c.Data.Rounding.Confirmed = false }},
		{"missing_slab", func(c *Configuration) { delete(c.Data.Grades[0].ProfessionalAllowance, "CA") }},
		{"missing_esi_basis", func(c *Configuration) { c.Data.ESI.ContributionBasis = "" }},
		{"missing_classification", func(c *Configuration) { c.Data.AnnualSpecialGroup = "" }},
		{"pending_policy", func(c *Configuration) { c.Data.Benefits[0].Kind = "pending" }},
		{"missing_policy", func(c *Configuration) { c.Data.Benefits = c.Data.Benefits[1:] }},
		{"incomplete_scope", func(c *Configuration) {
			c.Data.Locations = append(c.Data.Locations, Location{ID: "second", Name: "Second test location"})
			c.Data.Benefits[0].LocationID = c.Data.Locations[0].ID
		}},
		{"missing_address", func(c *Configuration) { c.Data.OfferTemplate.Address = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := publicationFixture()
			tc.change(&c)
			if len(ValidatePublication(c)) == 0 {
				t.Fatal("incomplete production policy was publishable")
			}
		})
	}
}

func TestFixedMoneyPrecisionCannotExceedPolicyOrRoundedCap(t *testing.T) {
	c := configuredFixture()
	c.Data.Grades[0].Conveyance = Str("7300.005")
	c.Data.Grades[0].ConveyanceCap = Str("7300.006")
	if len(ValidateConfig(c)) == 0 || Calculate(c, fixtureInput()).CoreValid {
		t.Fatal("fixed values that round above a cap must be rejected")
	}
}

func TestConflictingEligibilityConditionsAreRejected(t *testing.T) {
	c := configuredFixture()
	c.Data.Benefits[1].Conditions = []Condition{{Field: "relocation", Equals: true}, {Field: "relocation", Equals: false}}
	if len(ValidateConfig(c)) == 0 {
		t.Fatal("contradictory eligibility policy accepted")
	}
}

func TestConjunctiveEligibilityDoesNotDependOnConditionOrder(t *testing.T) {
	for _, conditions := range [][]Condition{
		{{Field: "relocation", Equals: true}, {Field: "fieldService", Equals: true}},
		{{Field: "fieldService", Equals: true}, {Field: "relocation", Equals: true}},
	} {
		c := configuredFixture()
		c.Data.Benefits[1] = BenefitRule{Component: "ANNUAL_SPECIAL", Kind: "fixed", Amount: "500", Period: "annual", Conditions: conditions}
		in := fixtureInput()
		in.Job["fieldService"] = false
		got := lineOf(Calculate(c, in), "ANNUAL_SPECIAL")
		if got.Status != "not_applicable" || got.Amount == nil || *got.Amount != "0.00" {
			t.Fatalf("known failed eligibility must yield zero regardless of condition order: %+v", got)
		}
	}
}

func TestCalculatedAmountsMustFitPersistedPrecision(t *testing.T) {
	c := configuredFixture()
	c.Data.Benefits[2] = BenefitRule{Component: "SIXTH_DAY", Kind: "percent", Rate: "999999999999", Basis: "gross", BasisPeriod: "annual", Period: "annual"}
	r := Calculate(c, fixtureInput())
	if r.CoreValid || r.CanFinalize {
		t.Fatal("amount exceeding database precision was accepted")
	}
	for _, issue := range r.Issues {
		if issue.Code == "AMOUNT_RANGE" {
			return
		}
	}
	t.Fatal("missing actionable amount-range error")
}
