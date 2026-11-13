package engine

import (
	"encoding/json"
	"testing"
)

func urdInput() Input {
	return Input{CandidateName: "URD Test Candidate", CandidateReference: "URD-TEST", GradeID: "urd-example", LocationID: "unconfirmed", Qualification: "NONE", Gross: "100000", Basic: "20000", TaxRegime: "new", NPSOptIn: true, CTCYear: 1, Job: map[string]bool{}}
}
func urdLine(t *testing.T, r Result, code string) Line {
	t.Helper()
	for _, line := range r.Lines {
		if line.Code == code {
			return line
		}
	}
	t.Fatalf("missing URD component %s", code)
	return Line{}
}
func urdAmount(t *testing.T, r Result, code, expected string) {
	t.Helper()
	line := urdLine(t, r, code)
	if line.Amount == nil || *line.Amount != expected {
		t.Fatalf("%s: got %v; want %s", code, line.Amount, expected)
	}
}
func TestURDWorkedExampleAndMissingRules(t *testing.T) {
	c := DemoConfiguration()
	r := Calculate(c, urdInput())
	if len(r.Lines) != 20 {
		t.Fatalf("expected all 20 URD components; got %d", len(r.Lines))
	}
	for code, amount := range map[string]string{"BASIC": "20000.00", "PROFESSIONAL": "0.00", "CONVEYANCE": "7300.00", "PF": "3000.00", "NPS": "2800.00", "SUPERANNUATION": "0.00", "SPECIAL": "30000.00", "HRA": "36900.00", "GRATUITY": "2405.00"} {
		urdAmount(t, r, code, amount)
	}
	if !r.CoreValid || r.Totals.Inclusion == nil || *r.Totals.Inclusion != "50000.00" || r.Totals.Exclusion == nil || *r.Totals.Exclusion != "50000.00" {
		t.Fatalf("URD parity failed: %+v issues=%+v", r.Totals, r.Issues)
	}
	if r.Complete || r.CanFinalize || r.Totals.AnnualCTC != nil {
		t.Fatal("unknown policies must not produce a complete CTC or final offer")
	}
	if r.Totals.KnownAnnualTotal != "1228860.00" {
		t.Fatalf("known subtotal double-counted/missing: %s", r.Totals.KnownAnnualTotal)
	}
	for _, code := range []string{"ESI", "ANNUAL_SPECIAL", "SIXTH_DAY", "FIELD_SERVICE", "RELOCATION", "DEPUTATION", "ADDITIONAL_HRA", "PERFORMANCE", "ADDITIONAL_PERFORMANCE", "LTM1", "LTM2"} {
		line := urdLine(t, r, code)
		if line.Amount != nil || line.Annual != nil || line.Status != "pending" {
			t.Fatalf("%s silently acquired an invented amount: %+v", code, line)
		}
	}
}
func TestURDPFTiersAndBoundary(t *testing.T) {
	cases := []struct{ gross, basic, pf string }{
		{"80000", "20000", "2400.00"},
		{"100000", "25000", "3000.00"},
		{"120000", "25000", "3000.00"},
		{"120000", "30000", "3600.00"},
		{"60000", "25000", "1800.00"},
		{"60000", "25001", "3000.12"},
	}
	for _, tc := range cases {
		t.Run(tc.gross+"_"+tc.basic, func(t *testing.T) {
			in := urdInput()
			in.Gross = tc.gross
			in.Basic = tc.basic
			r := Calculate(DemoConfiguration(), in)
			urdAmount(t, r, "PF", tc.pf)
		})
	}
}
func TestURDNPSRedistributesOnlyExclusion(t *testing.T) {
	cases := []struct {
		regime   string
		opt      bool
		nps, hra string
	}{{"new", true, "2800.00", "36900.00"}, {"old", true, "2000.00", "37700.00"}, {"new", false, "0.00", "39700.00"}}
	for _, tc := range cases {
		in := urdInput()
		in.TaxRegime = tc.regime
		in.NPSOptIn = tc.opt
		r := Calculate(DemoConfiguration(), in)
		urdAmount(t, r, "NPS", tc.nps)
		urdAmount(t, r, "HRA", tc.hra)
		if !r.CoreValid {
			t.Fatalf("NPS change broke parity: %+v", r.Issues)
		}
		if r.Totals.KnownAnnualTotal != "1228860.00" {
			t.Fatal("NPS counted again outside Gross")
		}
	}
}
func TestURDInvalidResidualsAndPrecisionBlockGeneration(t *testing.T) {
	cases := []struct{ name, gross, basic string }{{"negative_inclusion", "30000", "20000"}, {"negative_exclusion", "10000", "1000"}, {"odd_paise", "100000.01", "20000"}, {"zero_gross", "0", "20000"}, {"negative_basic", "100000", "-1"}, {"excess_precision", "100000", "20000.001"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := urdInput()
			in.Gross = tc.gross
			in.Basic = tc.basic
			r := Calculate(DemoConfiguration(), in)
			if r.CoreValid || r.CanFinalize {
				t.Fatalf("invalid scenario accepted: %+v", r)
			}
			found := false
			for _, i := range r.Issues {
				if i.Severity == "error" {
					found = true
				}
			}
			if !found {
				t.Fatal("invalid scenario did not explain its error")
			}
		})
	}
}
func TestURDQualifiedMissingSlabIsNotZero(t *testing.T) {
	in := urdInput()
	in.Qualification = "CA"
	r := Calculate(DemoConfiguration(), in)
	if urdLine(t, r, "PROFESSIONAL").Amount != nil || urdLine(t, r, "SPECIAL").Amount != nil || r.CoreValid {
		t.Fatal("missing professional slab was treated as a calculated zero")
	}
}
func TestURDGradeRangeValidation(t *testing.T) {
	c := DemoConfiguration()
	c.Data.Grades[0].BasicMin = Str("7000")
	c.Data.Grades[0].BasicMax = Str("12000")
	r := Calculate(c, urdInput())
	if r.CoreValid {
		t.Fatal("URD example Basic 20000 must not be accepted as Junior Engineer Basic in 7000-12000 range")
	}
}
func TestURDExplicitNPSElectionRequired(t *testing.T) {
	for _, payload := range []string{`{"gross":"100000"}`, `{"npsOptIn":null}`} {
		var in Input
		if json.Unmarshal([]byte(payload), &in) == nil {
			t.Fatal("missing NPS election silently became false")
		}
	}
}
func TestURDCalculationDoesNotMutateConfiguration(t *testing.T) {
	c := DemoConfiguration()
	before, _ := json.Marshal(c)
	Calculate(c, urdInput())
	after, _ := json.Marshal(c)
	if string(before) != string(after) {
		t.Fatal("calculation mutated its configuration version")
	}
}
