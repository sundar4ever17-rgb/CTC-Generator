package engine

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ValidateInput validates profile data independently from calculation policy.
// Blank employment details remain allowed for a draft; final documents apply
// their additional required-field checks. The explicit calculation date is not
// inferred from a joining date, because that convention is not in the URD.
func ValidateInput(in Input) []Issue {
	var issues []Issue
	add := func(field, message string) {
		issues = append(issues, Issue{Code: "INPUT_INVALID", Field: field, Message: message, Severity: "error"})
	}
	for _, field := range []struct {
		name, value string
		limit       int
	}{
		{"candidateName", in.CandidateName, 200},
		{"candidateReference", in.CandidateReference, 100},
		{"designation", in.Designation, 200},
		{"department", in.Department, 200},
		{"notes", in.Notes, 10000},
		{"gradeId", in.GradeID, 100},
		{"locationId", in.LocationID, 100},
	} {
		if utf8.RuneCountInString(field.value) > field.limit {
			add(field.name, fmt.Sprintf("%s must contain at most %d characters", field.name, field.limit))
		}
		if strings.ContainsFunc(field.value, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }) {
			add(field.name, field.name+" contains unsupported control characters")
		}
	}
	for _, field := range []struct{ name, value string }{{"joiningDate", in.JoiningDate}, {"calculationDate", in.CalculationDate}} {
		if field.value != "" {
			if _, err := time.Parse("2006-01-02", field.value); err != nil {
				add(field.name, field.name+" must be a valid YYYY-MM-DD date")
			}
		}
	}
	if in.CTCYear > 2147483647 {
		add("ctcYear", "CTC attribution year exceeds the supported integer range")
	}
	for field := range in.Job {
		if !jobField(field) {
			add("job", "Unsupported job parameter "+field)
		}
	}
	return issues
}

func validateConfigurationValues(c Configuration) []string {
	var errs []string
	text := func(label, value string, max int) {
		if utf8.RuneCountInString(value) > max || strings.ContainsRune(value, '\x00') {
			errs = append(errs, fmt.Sprintf("%s must contain at most %d characters and no null characters", label, max))
		}
	}
	text("Configuration name", c.Name, 200)
	text("Company name", c.Data.OfferTemplate.CompanyName, 300)
	text("Company address", c.Data.OfferTemplate.Address, 2000)
	text("Signatory", c.Data.OfferTemplate.Signatory, 300)
	text("Offer wording", c.Data.OfferTemplate.Body, 50000)
	// Bounds protect validation and configuration publication from unbounded
	// cross-product work. They are technical limits, not compensation policy.
	if len(c.Data.Grades) > 500 || len(c.Data.Locations) > 500 || len(c.Data.Benefits) > 5000 {
		errs = append(errs, "Configuration supports at most 500 grades, 500 locations and 5000 benefit rules")
	}
	money := func(label, value string) {
		if c.Data.Rounding.Scale >= 0 && c.Data.Rounding.Scale <= 2 && validDecimal(value) && !dec(value).Equal(dec(value).Round(c.Data.Rounding.Scale)) {
			errs = append(errs, label+" exceeds the configured money precision")
		}
	}
	for _, grade := range c.Data.Grades {
		text("Grade ID", grade.ID, 100)
		text("Grade name", grade.Name, 200)
		if strings.TrimSpace(grade.ID) == "" || strings.TrimSpace(grade.Name) == "" {
			errs = append(errs, "Grade ID and name cannot be blank")
		}
		for _, field := range []struct {
			name  string
			value *string
		}{{"Basic minimum", grade.BasicMin}, {"Basic maximum", grade.BasicMax}, {"Conveyance", grade.Conveyance}, {"Conveyance cap", grade.ConveyanceCap}} {
			if field.value != nil {
				money(field.name, *field.value)
			}
		}
		for _, amount := range grade.ProfessionalAllowance {
			money("Professional Allowance", amount)
		}
	}
	for _, location := range c.Data.Locations {
		text("Location ID", location.ID, 100)
		text("Location name", location.Name, 200)
		if strings.TrimSpace(location.ID) == "" || strings.TrimSpace(location.Name) == "" {
			errs = append(errs, "Location ID and name cannot be blank")
		}
	}
	for _, benefit := range c.Data.Benefits {
		text("Benefit notes", benefit.Notes, 10000)
		if benefit.Kind == "fixed" {
			money(benefit.Component+" amount", benefit.Amount)
		}
		if benefit.Cap != "" {
			money(benefit.Component+" cap", benefit.Cap)
		}
		for _, item := range benefit.Schedule {
			money(benefit.Component+" schedule amount", item.Amount)
			if item.Year > 2147483647 {
				errs = append(errs, "Schedule year exceeds the supported integer range")
			}
		}
	}
	return errs
}

// ValidatePublication is stricter than draft validation. Production publication
// is an explicit assertion by C&B that all master policies are supplied and
// approved. Missing values must not acquire illustrative defaults.
func ValidatePublication(c Configuration) []string {
	errs := ValidateConfig(c)
	if len(errs) != 0 {
		return errs
	}
	add := func(message string) { errs = append(errs, message) }
	d := c.Data
	if c.Source != "client" {
		add("Only client policy configurations may be published in production")
	}
	if c.EffectiveFrom == "" {
		add("An effective-from date is required for publication")
	}
	for _, approval := range []struct {
		name      string
		confirmed bool
	}{{"Basic selection", d.BasicMethodConfirmed}, {"Rounding", d.Rounding.Confirmed}, {"Annualization", d.AnnualizationConfirmed}, {"Uncapped HRA treatment", d.HRAApproved}, {"Offer template", d.TemplateApproved}} {
		if !approval.confirmed {
			add(approval.name + " must be explicitly approved before publication")
		}
	}
	if strings.TrimSpace(d.OfferTemplate.CompanyName) == "" || strings.TrimSpace(d.OfferTemplate.Address) == "" || strings.TrimSpace(d.OfferTemplate.Signatory) == "" || strings.TrimSpace(d.OfferTemplate.Body) == "" {
		add("The approved offer template requires company name, address, signatory and wording")
	}
	if d.AnnualSpecialGroup == "" {
		add("Annual Special Allowance classification must be explicitly selected")
	}
	if d.ESI.Ceiling == nil || d.ESI.CeilingInclusive == nil || d.ESI.EligibilityBasis == "" || d.ESI.ContributionBasis == "" {
		add("Complete ESI ceiling, boundary, eligibility and contribution bases before publication")
	}
	if len(d.Locations) == 0 {
		add("At least one location is required for publication")
	}
	for _, grade := range d.Grades {
		if grade.BasicMin == nil || grade.BasicMax == nil || grade.Conveyance == nil || grade.ConveyanceCap == nil {
			add("Complete the Basic range and Conveyance amount/cap for grade " + grade.ID)
		}
		for _, qualification := range []string{"CA", "CMA", "CS"} {
			if _, exists := grade.ProfessionalAllowance[qualification]; !exists {
				add("Configure the " + qualification + " Professional Allowance slab for grade " + grade.ID)
			}
		}
	}
	// Index grade/location applicability so validation stays linear in rule
	// count plus the supported grade/location combinations.
	type scope struct{ component, grade, location string }
	rules := map[scope]bool{}
	for _, rule := range d.Benefits {
		if rule.Kind == "pending" {
			add("Resolve pending policy " + rule.Component + " before publication")
			continue
		}
		rules[scope{rule.Component, rule.GradeID, rule.LocationID}] = true
	}
	for _, component := range []string{"SUPERANNUATION", "ANNUAL_SPECIAL", "SIXTH_DAY", "FIELD_SERVICE", "RELOCATION", "DEPUTATION", "ADDITIONAL_HRA", "PERFORMANCE", "ADDITIONAL_PERFORMANCE", "LTM1", "LTM2"} {
		missing := false
		for _, grade := range d.Grades {
			for _, location := range d.Locations {
				if !rules[scope{component, grade.ID, location.ID}] && !rules[scope{component, grade.ID, ""}] && !rules[scope{component, "", location.ID}] && !rules[scope{component, "", ""}] {
					missing = true
				}
			}
		}
		if missing {
			add(component + " requires a rule or explicit non-applicability for every grade/location combination")
		}
	}
	return errs
}
