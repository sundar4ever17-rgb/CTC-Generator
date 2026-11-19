package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ctc/internal/engine"
)

func ruleRequest(t *testing.T, path string, body any) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestRuleConversionAndPreviewAreUnsavedDrafts(t *testing.T) {
	// A nil DB deliberately proves these endpoints do not mutate persisted
	// versions, history or audit tables while the editor explores a policy.
	a := &App{Demo: true}
	c := engine.DemoConfiguration()
	c.ID, c.Version = "existing-version", 42
	before, _ := json.Marshal(c)
	s := session{User: User{Role: "cb"}}
	rr := httptest.NewRecorder()
	a.convertConfig(rr, ruleRequest(t, "/api/configurations/convert", c), s)
	if rr.Code != http.StatusOK {
		t.Fatalf("conversion failed: %d %s", rr.Code, rr.Body.String())
	}
	var converted engine.Configuration
	if err := json.Unmarshal(rr.Body.Bytes(), &converted); err != nil {
		t.Fatal(err)
	}
	if converted.ID != "" || converted.Version != 0 || converted.Status != "draft" || converted.Data.Rules == nil {
		t.Fatalf("conversion did not return an unsaved rule draft: %+v", converted)
	}
	after, _ := json.Marshal(c)
	if !bytes.Equal(before, after) {
		t.Fatal("conversion changed the source configuration")
	}
	converted.ID, converted.Version, converted.Status = "pretend-saved", 8, "published"
	in := engine.Input{Gross: "100000", Basic: "20000", GradeID: "urd-example", LocationID: "unconfirmed", Qualification: "NONE", TaxRegime: "new", NPSOptIn: true, CTCYear: 1}
	rr = httptest.NewRecorder()
	a.previewConfig(rr, ruleRequest(t, "/api/configurations/preview", map[string]any{"configuration": converted, "input": in}), s)
	if rr.Code != http.StatusOK {
		t.Fatalf("preview failed: %d %s", rr.Code, rr.Body.String())
	}
	var result engine.Result
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.CanFinalize || result.ConfigurationID != "" || result.ConfigurationVersion != 0 || result.EngineVersion != engine.RuleEngineVersion {
		t.Fatalf("preview is not an unsaved v2 result: %+v", result)
	}
	if !result.CoreValid || result.Totals.Inclusion == nil || *result.Totals.Inclusion != "50000.00" {
		t.Fatalf("preview changed the worked example: %+v", result)
	}
}

func TestRulePreviewKeepsProductionSourceAndDateGuards(t *testing.T) {
	a := &App{}
	c := engine.ConvertConfiguration(engine.DemoConfiguration())
	s := session{User: User{Role: "cb"}}
	rr := httptest.NewRecorder()
	a.previewConfig(rr, ruleRequest(t, "/api/configurations/preview", map[string]any{"configuration": c, "input": engine.Input{}}), s)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("production accepted demo policy: %d %s", rr.Code, rr.Body.String())
	}
	c.Source = "client"
	rr = httptest.NewRecorder()
	a.previewConfig(rr, ruleRequest(t, "/api/configurations/preview", map[string]any{"configuration": c, "input": engine.Input{}}), s)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "effective date") {
		t.Fatalf("production preview accepted absent date: %d %s", rr.Code, rr.Body.String())
	}
}

func TestDocumentUsesSavedDynamicGroupsAndTargets(t *testing.T) {
	c := engine.DemoConfiguration()
	v := Revision{Result: engine.Result{
		Groups: []engine.GroupResult{
			{Code: "inclusion", Name: "Inclusion wages", InGross: true, Monthly: engine.Str("60000.00"), Target: engine.Str("60000.00"), Difference: engine.Str("0.00"), Valid: true},
			{Code: "exclusion", Name: "Exclusion wages", InGross: true, Monthly: engine.Str("40000.00"), Target: engine.Str("40000.00"), Difference: engine.Str("0.00"), Valid: true},
			{Code: "wellbeing", Name: "Wellbeing <script>", Annual: engine.Str("12000.00"), Valid: true},
		},
		Lines:  []engine.Line{{Code: "WELLNESS", Name: "Wellness benefit", Group: "wellbeing", Annual: engine.Str("12000.00"), Period: "annual", Status: "calculated"}},
		Totals: engine.Totals{Gross: "100000.00", KnownAnnualTotal: "1212000.00"},
	}}
	html, err := renderDocument(v, c, c.Data.OfferTemplate, "ctc", "draft")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Wellness benefit", "Wellbeing &lt;script&gt;", "60000.00", "40000.00", "target / difference", "12000.00"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("saved category/target missing from document: %s", expected)
		}
	}
	if strings.Contains(html, "Parity difference") || strings.Contains(html, "<script>") {
		t.Fatal("dynamic document assumes equality or includes unescaped labels")
	}
}
