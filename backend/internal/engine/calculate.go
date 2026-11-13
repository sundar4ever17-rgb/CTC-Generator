package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func (i *Input) UnmarshalJSON(b []byte) error {
	type plain Input
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var required map[string]json.RawMessage
	if err := json.Unmarshal(b, &required); err != nil {
		return err
	}
	raw, ok := required["npsOptIn"]
	if !ok || string(raw) == "null" {
		return fmt.Errorf("npsOptIn must explicitly be true or false")
	}
	*i = Input(v)
	return nil
}

var decimalPattern = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,8})?$`)

func validDecimal(s string) bool { return decimalPattern.MatchString(s) }
func dec(s string) decimal.Decimal {
	if len(s) > 40 || strings.ContainsAny(s, "eE") {
		return decimal.Zero
	}
	d, _ := decimal.NewFromString(s)
	return d
}
func allowedBasis(s string, hra bool) bool {
	return s == "basic" || s == "gross" || s == "inclusion" || s == "exclusion" || (hra && s == "hra")
}
func ValidateConfig(c Configuration) []string {
	d := c.Data
	var errs []string
	add := func(s string) { errs = append(errs, s) }
	if strings.TrimSpace(c.Name) == "" {
		add("Configuration name is required")
	}
	if c.Source != "client" && c.Source != "urd-demo" {
		add("Source must be client or urd-demo")
	}
	if d.BasicMethod != "manual" {
		add("Only manual Basic selection is defined for this demo; another method requires an approved rule")
	}
	if d.Rounding.Scale < 0 || d.Rounding.Scale > 2 || d.Rounding.Mode != "half_up" {
		add("Choose a 0-2 decimal, half_up rounding policy; another convention requires a reviewed rule")
	}
	for name, v := range map[string]string{"PF threshold": d.PF.BasicThreshold, "PF inclusion share": d.PF.InclusionShare, "PF cap": d.PF.BasisCap, "PF rate": d.PF.Rate, "NPS old rate": d.NPS.OldRate, "NPS new rate": d.NPS.NewRate, "ESI rate": d.ESI.Rate, "Gratuity rate": d.GratuityRate} {
		if !validDecimal(v) {
			add(name + " must be a nonnegative decimal below 1 trillion")
		}
	}
	for name, v := range map[string]string{"PF inclusion share": d.PF.InclusionShare, "PF rate": d.PF.Rate, "NPS old rate": d.NPS.OldRate, "NPS new rate": d.NPS.NewRate, "ESI rate": d.ESI.Rate, "Gratuity rate": d.GratuityRate} {
		if validDecimal(v) && dec(v).GreaterThan(decimal.NewFromInt(1)) {
			add(name + " must be a fraction between 0 and 1")
		}
	}
	if d.ESI.Ceiling != nil && !validDecimal(*d.ESI.Ceiling) {
		add("ESI ceiling must be a nonnegative decimal")
	}
	if d.ESI.EligibilityBasis != "" && !allowedBasis(d.ESI.EligibilityBasis, false) {
		add("Unsupported ESI eligibility basis")
	}
	if d.ESI.ContributionBasis != "" && !allowedBasis(d.ESI.ContributionBasis, false) {
		add("Unsupported ESI contribution basis")
	}
	if d.AnnualSpecialGroup != "" && d.AnnualSpecialGroup != "statutory" && d.AnnualSpecialGroup != "policy" {
		add("Annual Special Allowance group must be statutory or policy")
	}
	for _, v := range []string{c.EffectiveFrom, c.EffectiveTo} {
		if v != "" {
			if _, e := time.Parse("2006-01-02", v); e != nil {
				add("Effective dates must be YYYY-MM-DD")
			}
		}
	}
	if c.EffectiveFrom != "" && c.EffectiveTo != "" && c.EffectiveFrom > c.EffectiveTo {
		add("Effective from must not exceed effective to")
	}
	grades := map[string]bool{}
	locations := map[string]bool{}
	if len(d.Grades) == 0 {
		add("At least one grade or explicitly labeled illustration is required")
	}
	for _, g := range d.Grades {
		if g.ID == "" || g.Name == "" || grades[g.ID] {
			add("Grades require unique IDs and names")
		}
		grades[g.ID] = true
		for _, v := range []*string{g.BasicMin, g.BasicMax, g.Conveyance, g.ConveyanceCap} {
			if v != nil && !validDecimal(*v) {
				add("Grade amounts must be nonnegative decimals")
			}
		}
		if g.BasicMin != nil && g.BasicMax != nil && dec(*g.BasicMin).GreaterThan(dec(*g.BasicMax)) {
			add("Basic minimum exceeds maximum for " + g.Name)
		}
		if g.Conveyance != nil && g.ConveyanceCap != nil && validDecimal(*g.Conveyance) && validDecimal(*g.ConveyanceCap) && dec(*g.Conveyance).GreaterThan(dec(*g.ConveyanceCap)) {
			add("Conveyance exceeds grade cap")
		}
		for q, v := range g.ProfessionalAllowance {
			if q != "CA" && q != "CMA" && q != "CS" {
				add("Professional Allowance supports CA, CMA and CS only")
			}
			if !validDecimal(v) {
				add("Professional Allowance slab must be a nonnegative decimal")
			}
		}
	}
	for _, l := range d.Locations {
		if l.ID == "" || l.Name == "" || locations[l.ID] {
			add("Locations require unique IDs and names")
		}
		locations[l.ID] = true
	}
	benefits := map[string]bool{}
	for _, code := range []string{"SUPERANNUATION", "ANNUAL_SPECIAL", "SIXTH_DAY", "FIELD_SERVICE", "RELOCATION", "DEPUTATION", "ADDITIONAL_HRA", "PERFORMANCE", "ADDITIONAL_PERFORMANCE", "LTM1", "LTM2"} {
		benefits[code] = true
	}
	for i, r := range d.Benefits {
		if !benefits[r.Component] {
			add("Unknown configurable benefit " + r.Component)
		}
		if r.GradeID != "" && !grades[r.GradeID] {
			add("Benefit references an unknown grade")
		}
		if r.LocationID != "" && !locations[r.LocationID] {
			add("Benefit references an unknown location")
		}
		if r.Kind != "pending" && r.Kind != "none" && r.Kind != "fixed" && r.Kind != "percent" && r.Kind != "schedule" {
			add("Unsupported benefit rule kind")
		}
		if r.Kind != "pending" && r.Period != "monthly" && r.Period != "annual" && r.Period != "one_time" {
			add("Configured benefits need an explicit payment period")
		}
		if r.Component == "SUPERANNUATION" && r.Kind != "pending" && r.Period != "monthly" {
			add("Superannuation must have a monthly allocation inside Gross")
		}
		if r.Component == "ANNUAL_SPECIAL" && r.Kind != "pending" && r.Period != "annual" {
			add("Annual Special Allowance must be annual")
		}
		if r.Component == "SUPERANNUATION" && r.Kind != "pending" && r.Kind != "none" && r.Kind != "percent" {
			add("Superannuation must use a percentage rule or explicit non-applicability")
		}
		if r.Component == "ANNUAL_SPECIAL" && r.Kind != "pending" && r.Kind != "none" && r.Kind != "fixed" {
			add("Annual Special Allowance must use an annual fixed slab")
		}
		if r.Kind == "percent" && (r.BasisPeriod != "monthly" && r.BasisPeriod != "annual") {
			add(r.Component + " percentage rule requires an explicit basisPeriod")
		}
		if r.Component == "SUPERANNUATION" && r.Kind == "percent" && r.BasisPeriod != "monthly" {
			add("Superannuation requires a monthly upstream basis")
		}
		if r.Kind == "fixed" && !validDecimal(r.Amount) {
			add(r.Component + " needs a nonnegative fixed amount")
		}
		if r.Kind == "percent" && (!validDecimal(r.Rate) || !allowedBasis(r.Basis, r.Component != "SUPERANNUATION")) {
			add(r.Component + " needs a valid rate and upstream basis")
		}
		if r.Cap != "" && !validDecimal(r.Cap) {
			add(r.Component + " cap must be a nonnegative decimal")
		}
		if r.Kind == "schedule" {
			if r.Component == "SUPERANNUATION" || r.Period != "annual" {
				add("Schedules require annual attribution and cannot determine monthly Superannuation")
			}
			if len(r.Schedule) == 0 {
				add("Schedule needs at least one year")
			}
			years := map[int]bool{}
			for _, s := range r.Schedule {
				if s.Year < 1 || years[s.Year] || !validDecimal(s.Amount) {
					add("Schedule years must be unique positive integers with valid amounts")
				}
				years[s.Year] = true
			}
		}
		for _, condition := range r.Conditions {
			if condition.Field != "sixthDay" && condition.Field != "fieldService" && condition.Field != "relocation" && condition.Field != "deputation" && condition.Field != "additionalHra" {
				add("Unsupported job condition " + condition.Field)
			}
		}
		for j := 0; j < i; j++ {
			other := d.Benefits[j]
			if other.Component == r.Component && (other.GradeID == r.GradeID || other.GradeID == "" || r.GradeID == "") && (other.LocationID == r.LocationID || other.LocationID == "" || r.LocationID == "") {
				add("Overlapping benefit rules for " + r.Component + "; use unambiguous grade/location matches")
			}
		}
	}
	return errs
}

func Calculate(c Configuration, in Input) Result {
	out := Result{ConfigurationID: c.ID, ConfigurationVersion: c.Version, EngineVersion: Version, Lines: []Line{}, Issues: []Issue{}}
	scale := c.Data.Rounding.Scale
	if scale < 0 || scale > 2 {
		scale = 2
	}
	money := func(d decimal.Decimal) string { return d.Round(scale).StringFixed(scale) }
	idx := map[string]int{}
	values := map[string]decimal.Decimal{}
	known := map[string]bool{}
	for _, component := range Components {
		group := component.Group
		if component.Code == "ANNUAL_SPECIAL" && c.Data.AnnualSpecialGroup == "policy" {
			group = "policy"
		}
		idx[component.Code] = len(out.Lines)
		out.Lines = append(out.Lines, Line{Code: component.Code, Name: component.Name, Group: group, Period: "", Status: "pending", Formula: "Required configuration is missing"})
	}
	issue := func(code, field, message, severity string) {
		out.Issues = append(out.Issues, Issue{code, field, message, severity})
	}
	set := func(code string, v decimal.Decimal, period, formula, status string) {
		v = v.Round(scale)
		l := &out.Lines[idx[code]]
		l.Period = period
		l.Amount = Str(money(v))
		l.Status = status
		l.Formula = formula
		if period == "monthly" {
			l.Monthly = Str(money(v))
			l.Annual = Str(money(v.Mul(decimal.NewFromInt(12))))
		} else {
			l.Annual = Str(money(v))
			l.Monthly = nil
		}
		values[code] = v
		known[code] = true
	}
	pending := func(code, msg string) {
		out.Lines[idx[code]].Formula = msg
		issue("MISSING_"+code, code, msg, "pending")
	}
	configErrors := ValidateConfig(c)
	for _, e := range configErrors {
		issue("CONFIG_INVALID", "configuration", e, "error")
	}
	if len(configErrors) > 0 {
		return out
	}
	if !validDecimal(in.Gross) || !validDecimal(in.Basic) {
		issue("INPUT_DECIMAL", "gross", "Gross and Basic must be plain nonnegative decimal strings with at most 12 integer and 8 fractional digits", "error")
		return out
	}
	gross, gErr := decimal.NewFromString(in.Gross)
	basic, bErr := decimal.NewFromString(in.Basic)
	if gErr != nil || !gross.IsPositive() || gross.GreaterThanOrEqual(decimal.NewFromInt(1000000000000)) {
		issue("GROSS_INVALID", "gross", "Monthly Gross must be positive and below 1 trillion", "error")
	}
	if bErr != nil || basic.IsNegative() || basic.GreaterThanOrEqual(decimal.NewFromInt(1000000000000)) {
		issue("BASIC_INVALID", "basic", "Basic must be a nonnegative amount below 1 trillion", "error")
	}
	if gErr != nil || bErr != nil || !gross.IsPositive() || basic.IsNegative() {
		return out
	}
	out.Totals.Gross = money(gross)
	if !gross.Equal(gross.Round(scale)) || !basic.Equal(basic.Round(scale)) {
		issue("INPUT_PRECISION", "gross", "Input money exceeds the selected rounding precision", "error")
	}
	target := gross.Div(decimal.NewFromInt(2))
	if !target.Equal(target.Round(scale)) {
		issue("GROSS_UNSPLITTABLE", "gross", "Gross cannot split equally at the configured precision; adjust Gross to an even minor-unit amount", "error")
	}
	var grade *Grade
	for j := range c.Data.Grades {
		if c.Data.Grades[j].ID == in.GradeID {
			grade = &c.Data.Grades[j]
		}
	}
	if grade == nil {
		issue("GRADE_INVALID", "gradeId", "Select a configured grade", "error")
		return out
	}
	locFound := false
	for _, loc := range c.Data.Locations {
		if loc.ID == in.LocationID {
			locFound = true
		}
	}
	if !locFound {
		issue("LOCATION_INVALID", "locationId", "Select a configured location", "error")
	}
	if in.TaxRegime != "new" && in.TaxRegime != "old" {
		issue("REGIME_INVALID", "taxRegime", "Tax regime must be new or old", "error")
	}
	if in.Qualification != "NONE" && in.Qualification != "CA" && in.Qualification != "CMA" && in.Qualification != "CS" {
		issue("QUALIFICATION_INVALID", "qualification", "Choose NONE, CA, CMA or CS", "error")
	}
	if in.CTCYear < 1 {
		issue("YEAR_INVALID", "ctcYear", "CTC attribution year must be a positive integer", "error")
	}
	if grade.BasicMin == nil || grade.BasicMax == nil {
		issue("BASIC_RANGE_PENDING", "basic", "Grade-wise Basic minimum/maximum have not been supplied", "pending")
	} else if basic.LessThan(dec(*grade.BasicMin)) || basic.GreaterThan(dec(*grade.BasicMax)) {
		issue("BASIC_RANGE", "basic", "Basic is outside the configured grade range", "error")
	}
	if !c.Data.BasicMethodConfirmed {
		issue("BASIC_METHOD_PENDING", "basic", "Manual Basic selection is demonstrated; client confirmation is pending", "pending")
	}
	if !c.Data.Rounding.Confirmed {
		issue("ROUNDING_PENDING", "configuration", "Two-decimal/selected half-up rounding is a demonstration convention until confirmed", "pending")
	}
	if !c.Data.AnnualizationConfirmed {
		issue("ANNUALIZATION_PENDING", "configuration", "Annual values use 12 monthly periods; client annualization approval is pending", "pending")
	}
	if !c.Data.HRAApproved {
		issue("HRA_APPROVAL_PENDING", "configuration", "Finance/Tax approval of uncapped HRA treatment is pending", "pending")
	}
	set("BASIC", basic, "monthly", "Selected Basic; validate against the grade range", "calculated")
	if in.Qualification == "NONE" {
		set("PROFESSIONAL", decimal.Zero, "monthly", "No recognized professional qualification", "not_applicable")
	} else if amount, ok := grade.ProfessionalAllowance[in.Qualification]; ok {
		set("PROFESSIONAL", dec(amount), "monthly", "Approved "+in.Qualification+" slab for the grade", "calculated")
	} else {
		pending("PROFESSIONAL", "Professional qualification is eligible, but its slab is not configured")
	}
	if grade.ConveyanceCap == nil {
		issue("CONVEYANCE_CAP_PENDING", "configuration", "The grade Conveyance cap has not been supplied", "pending")
	}
	if grade.Conveyance == nil {
		pending("CONVEYANCE", "The grade Conveyance amount has not been supplied")
	} else {
		set("CONVEYANCE", dec(*grade.Conveyance), "monthly", "Configured grade flat amount", "calculated")
	}
	pf := c.Data.PF
	if basic.GreaterThan(dec(pf.BasicThreshold)) {
		set("PF", basic.Mul(dec(pf.Rate)), "monthly", "Basic exceeds threshold: Basic x PF rate", "calculated")
	} else {
		basis := target.Mul(dec(pf.InclusionShare))
		if basis.GreaterThan(dec(pf.BasisCap)) {
			basis = dec(pf.BasisCap)
		}
		set("PF", basis.Mul(dec(pf.Rate)), "monthly", "Lower-Basic branch: min(Inclusion target x share, basis cap) x rate", "calculated")
	}
	if !in.NPSOptIn {
		set("NPS", decimal.Zero, "monthly", "NPS explicitly opted out", "not_applicable")
	} else {
		rate := c.Data.NPS.NewRate
		if in.TaxRegime == "old" {
			rate = c.Data.NPS.OldRate
		}
		set("NPS", basic.Mul(dec(rate)), "monthly", "Basic x "+rate+" ("+in.TaxRegime+" regime)", "calculated")
	}
	bases := map[string]decimal.Decimal{"basic": basic, "gross": gross, "inclusion": target, "exclusion": target}
	calculateBenefit := func(code string) {
		var rule *BenefitRule
		for j := range c.Data.Benefits {
			r := &c.Data.Benefits[j]
			if r.Component == code && (r.GradeID == "" || r.GradeID == in.GradeID) && (r.LocationID == "" || r.LocationID == in.LocationID) {
				if rule != nil {
					pending(code, "Multiple rules match; configuration must be unambiguous")
					return
				}
				rule = r
			}
		}
		if rule == nil || rule.Kind == "pending" {
			pending(code, "Client rule has not been supplied")
			return
		}
		for _, condition := range rule.Conditions {
			actual, exists := in.Job[condition.Field]
			if !exists {
				pending(code, "Required job parameter "+condition.Field+" was not provided")
				return
			}
			if actual != condition.Equals {
				set(code, decimal.Zero, rule.Period, "Approved eligibility condition not met", "not_applicable")
				return
			}
		}
		var amount decimal.Decimal
		formula := ""
		switch rule.Kind {
		case "none":
			set(code, decimal.Zero, rule.Period, "Explicitly not applicable: "+rule.Notes, "not_applicable")
			return
		case "fixed":
			amount = dec(rule.Amount)
			formula = "Configured " + rule.Period + " amount"
		case "percent":
			base, ok := bases[rule.Basis]
			if !ok {
				pending(code, "Required upstream basis "+rule.Basis+" is unresolved")
				return
			}
			if rule.BasisPeriod == "annual" {
				if !c.Data.AnnualizationConfirmed {
					pending(code, "Annual percentage basis requires annualization confirmation")
					return
				}
				base = base.Mul(decimal.NewFromInt(12))
			}
			amount = base.Mul(dec(rule.Rate))
			formula = rule.BasisPeriod + " " + rule.Basis + " x " + rule.Rate
		case "schedule":
			found := false
			for _, entry := range rule.Schedule {
				if entry.Year == in.CTCYear {
					amount = dec(entry.Amount)
					found = true
				}
			}
			if !found {
				pending(code, "No explicit schedule amount exists for this CTC year")
				return
			}
			formula = fmt.Sprintf("Approved schedule amount for year %d", in.CTCYear)
		default:
			pending(code, "Unsupported benefit rule")
			return
		}
		if rule.Cap != "" && amount.GreaterThan(dec(rule.Cap)) {
			amount = dec(rule.Cap)
			formula += " (capped)"
		}
		set(code, amount, rule.Period, formula, "calculated")
	}
	calculateBenefit("SUPERANNUATION")
	if known["PROFESSIONAL"] {
		special := target.Sub(basic).Sub(values["PROFESSIONAL"])
		set("SPECIAL", special, "monthly", "Gross / 2 - Basic - Professional Allowance", "calculated")
		if special.IsNegative() {
			issue("NEGATIVE_SPECIAL", "basic", "Fixed Inclusion components exceed Gross / 2; Special Allowance is negative", "error")
		}
	} else {
		pending("SPECIAL", "Professional Allowance must be resolved first")
	}
	if known["CONVEYANCE"] && known["SUPERANNUATION"] {
		hra := target.Sub(values["CONVEYANCE"]).Sub(values["PF"]).Sub(values["NPS"]).Sub(values["SUPERANNUATION"])
		set("HRA", hra, "monthly", "Gross / 2 - Conveyance - PF - NPS - Superannuation; no cap relative to Basic", "calculated")
		bases["hra"] = hra
		if hra.IsNegative() {
			issue("NEGATIVE_HRA", "gross", "Fixed Exclusion components exceed Gross / 2; HRA is negative", "error")
		}
	} else {
		pending("HRA", "All fixed Exclusion components must be resolved first")
	}
	esi := c.Data.ESI
	if esi.Ceiling == nil || esi.CeilingInclusive == nil || esi.EligibilityBasis == "" || esi.ContributionBasis == "" {
		pending("ESI", "ESI ceiling, boundary comparison, eligibility basis and contribution basis require client confirmation")
	} else {
		covered := bases[esi.EligibilityBasis].LessThan(dec(*esi.Ceiling))
		if *esi.CeilingInclusive && bases[esi.EligibilityBasis].Equal(dec(*esi.Ceiling)) {
			covered = true
		}
		if covered {
			set("ESI", bases[esi.ContributionBasis].Mul(dec(esi.Rate)), "monthly", "Covered wages within configured ceiling; contribution basis x ESI rate", "calculated")
		} else {
			set("ESI", decimal.Zero, "monthly", "Eligibility wages exceed configured ESI ceiling", "not_applicable")
		}
	}
	set("GRATUITY", target.Mul(dec(c.Data.GratuityRate)), "monthly", "Inclusion target x Gratuity rate (CTC provision)", "calculated")
	for _, code := range []string{"ANNUAL_SPECIAL", "SIXTH_DAY", "FIELD_SERVICE", "RELOCATION", "DEPUTATION", "ADDITIONAL_HRA", "PERFORMANCE", "ADDITIONAL_PERFORMANCE", "LTM1", "LTM2"} {
		calculateBenefit(code)
	}
	groupMonthly := map[string]decimal.Decimal{}
	groupAnnual := map[string]decimal.Decimal{}
	groupKnown := map[string]bool{"inclusion": true, "exclusion": true, "statutory": true, "policy": true, "variable": true}
	allKnown := true
	knownAnnual := decimal.Zero
	for _, line := range out.Lines {
		if line.Annual == nil {
			groupKnown[line.Group] = false
			allKnown = false
			continue
		}
		a := dec(*line.Annual)
		groupAnnual[line.Group] = groupAnnual[line.Group].Add(a)
		knownAnnual = knownAnnual.Add(a)
		if line.Monthly != nil {
			groupMonthly[line.Group] = groupMonthly[line.Group].Add(dec(*line.Monthly))
		}
	}
	out.Totals.KnownAnnualTotal = money(knownAnnual)
	if groupKnown["inclusion"] {
		out.Totals.Inclusion = Str(money(groupMonthly["inclusion"]))
	}
	if groupKnown["exclusion"] {
		out.Totals.Exclusion = Str(money(groupMonthly["exclusion"]))
	}
	if groupKnown["statutory"] {
		out.Totals.StatutoryAnnual = Str(money(groupAnnual["statutory"]))
	}
	if groupKnown["policy"] {
		out.Totals.PolicyAnnual = Str(money(groupAnnual["policy"]))
	}
	if groupKnown["variable"] {
		out.Totals.VariableAnnual = Str(money(groupAnnual["variable"]))
	}
	hasError := false
	hasPending := false
	for _, i := range out.Issues {
		if i.Severity == "error" {
			hasError = true
		}
		if i.Severity == "pending" {
			hasPending = true
		}
	}
	if out.Totals.Inclusion != nil && out.Totals.Exclusion != nil {
		i, e := groupMonthly["inclusion"], groupMonthly["exclusion"]
		out.Totals.Difference = Str(money(i.Sub(e)))
		out.CoreValid = !hasError && i.Equal(target) && e.Equal(target) && i.Add(e).Equal(gross) && !values["SPECIAL"].IsNegative() && !values["HRA"].IsNegative()
		if !i.Equal(target) || !e.Equal(target) || !i.Add(e).Equal(gross) {
			issue("PARITY_FAILED", "gross", "Group totals must equal Gross / 2 and reconcile exactly to Gross", "error")
		}
	}
	out.Complete = out.CoreValid && allKnown && !hasPending
	if out.Complete {
		out.Totals.AnnualCTC = Str(money(knownAnnual))
	}
	out.CanFinalize = out.Complete && c.Source == "client" && c.Status == "published" && c.Data.TemplateApproved && strings.TrimSpace(c.Data.OfferTemplate.CompanyName) != "" && strings.TrimSpace(c.Data.OfferTemplate.Body) != "" && strings.TrimSpace(c.Data.OfferTemplate.Signatory) != ""
	if c.Source != "client" {
		issue("DEMO_CONFIGURATION", "configuration", "URD illustrative configuration: outputs are demonstration drafts only", "warning")
	}
	if c.Status != "published" {
		issue("DRAFT_CONFIGURATION", "configuration", "Publish this configuration before final documents can be generated", "warning")
	}
	if !c.Data.TemplateApproved {
		issue("TEMPLATE_PENDING", "configuration", "Offer wording and company fields require approval; final documents are blocked", "warning")
	}
	return out
}
