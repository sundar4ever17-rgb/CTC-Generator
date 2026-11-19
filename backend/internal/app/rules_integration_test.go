package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in integration test only accepts a new, empty isolated *_test DB.
// It first constructs the previous schema, then tests a non-destructive upgrade.
func TestRuleMigrationPreservesHistoryAndPersistsDynamicPolicy(t *testing.T) {
	raw := os.Getenv("RULES_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set RULES_TEST_DATABASE_URL to a new empty isolated *_test database")
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(u.Path, "_test") || os.Getenv("CTC_TEST_FIXTURES") != "1" {
		t.Fatal("requires CTC_TEST_FIXTURES=1 and an isolated *_test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("requires an empty database; existing data preserved: tables=%d error=%v", count, err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE schema_migrations(version text PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())")
	for _, migration := range migrations()[:3] {
		exec(migration.sql)
		exec("INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)", migration.version, hash(strings.ReplaceAll(migration.sql, "\r\n", "\n")))
	}
	c := engine.DemoConfiguration()
	c.ID = "legacy-policy"
	data, _ := json.Marshal(c.Data)
	exec("INSERT INTO users(id,name,email,password_hash,role) VALUES('fixture-cb','Fixture C&B','cb@fixture.local','unused','cb')")
	exec("INSERT INTO configurations(id,name,source,status,data) VALUES($1,$2,$3,'published',$4)", c.ID, c.Name, c.Source, data)
	for i, component := range engine.Components {
		exec("INSERT INTO salary_components(code,name) VALUES($1,$2)", component.Code, component.Name)
		exec("INSERT INTO component_configurations(configuration_id,component_code,group_code,display_order) VALUES($1,$2,$3,$4)", c.ID, component.Code, component.Group, i)
	}
	exec("INSERT INTO grade_salary_configurations(configuration_id,grade_id,name,basic_selection_method) VALUES($1,'urd-example','URD example','manual')", c.ID)
	exec("INSERT INTO location_configurations(configuration_id,location_id,name) VALUES($1,'unconfirmed','Unconfirmed')", c.ID)
	in := engine.Input{CandidateName: "Historical candidate", Gross: "100000", Basic: "20000", GradeID: "urd-example", LocationID: "unconfirmed", Qualification: "NONE", TaxRegime: "new", NPSOptIn: true, CTCYear: 1}
	legacy := engine.Calculate(c, in)
	input, _ := json.Marshal(in)
	result, _ := json.Marshal(legacy)
	exec("INSERT INTO candidates(id,candidate_reference,name,profile_snapshot) VALUES('legacy-candidate','HIST-1','Historical candidate',$1)", input)
	exec("INSERT INTO ctc_cases(id,candidate_id,owner_user_id) VALUES('legacy-case','legacy-candidate','fixture-cb')")
	exec(`INSERT INTO ctc_revisions(id,case_id,revision_number,configuration_id,engine_version,grade_id,location_id,qualification_code,gross,basic,tax_regime,nps_opt_in,input_snapshot,result_snapshot,status,created_by) VALUES('legacy-revision','legacy-case',1,$1,$2,'urd-example','unconfirmed','NONE',100000,20000,'new',true,$3,$4,'incomplete','fixture-cb')`, c.ID, legacy.EngineVersion, input, result)
	template, _ := json.Marshal(c.Data.OfferTemplate)
	exec("INSERT INTO document_template_versions(id,configuration_id,document_type,template_snapshot) VALUES('legacy-template',$1,'ctc',$2)", c.ID, template)
	pdf := []byte("%PDF preserved historical fixture")
	exec("INSERT INTO generated_documents(id,revision_id,template_version_id,document_type,mode,content,checksum,generated_by) VALUES('legacy-document','legacy-revision','legacy-template','ctc','draft',$1,$2,'fixture-cb')", pdf, hash(string(pdf)))
	var oldData, oldResult []byte
	if err = pool.QueryRow(ctx, "SELECT c.data,r.result_snapshot FROM configurations c JOIN ctc_revisions r ON r.configuration_id=c.id WHERE r.id='legacy-revision'").Scan(&oldData, &oldResult); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var newData, newResult []byte
	if err = pool.QueryRow(ctx, "SELECT c.data,r.result_snapshot FROM configurations c JOIN ctc_revisions r ON r.configuration_id=c.id WHERE r.id='legacy-revision'").Scan(&newData, &newResult); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldData, newData) || !bytes.Equal(oldResult, newResult) {
		t.Fatal("migration rewrote historical policy or calculation")
	}
	a := &App{DB: pool, Demo: true}
	s := session{User: User{ID: "fixture-cb", Role: "cb"}}
	r := httptest.NewRequest(http.MethodGet, "/api/revisions/legacy-revision/documents/ctc", nil).WithContext(ctx)
	r.SetPathValue("id", "legacy-revision")
	r.SetPathValue("kind", "ctc")
	rr := httptest.NewRecorder()
	a.document(rr, r, s)
	if rr.Code != http.StatusOK || !bytes.Equal(rr.Body.Bytes(), pdf) {
		t.Fatalf("historical PDF changed or became unavailable: %d %s", rr.Code, rr.Body.String())
	}
	v2 := engine.ConvertConfiguration(c)
	v2.Data.Rules.Groups = append(v2.Data.Rules.Groups, engine.GroupDefinition{Code: "wellbeing", Name: "Wellbeing"})
	v2.Data.Rules.Components = append(v2.Data.Rules.Components, engine.ComponentDefinition{Code: "WELLNESS", Name: "Wellness benefit", Group: "wellbeing", Rules: []engine.CalculationRule{{Period: "monthly", Status: "calculated", Expression: &engine.Expression{Op: "number", Value: "100"}}}})
	if problems := engine.ValidateConfig(v2); len(problems) > 0 {
		t.Fatalf("dynamic fixture invalid: %v", problems)
	}
	v2, err = a.insertConfig(ctx, v2, s.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := a.config(ctx, v2.ID)
	if err != nil || loaded.Data.Rules == nil || len(loaded.Data.Rules.Components) != len(engine.Components)+1 {
		t.Fatalf("dynamic rules did not round-trip: %v", err)
	}
	rr = httptest.NewRecorder()
	a.saveCase(rr, ruleRequest(t, "/api/cases", calculationRequest{ConfigurationID: v2.ID, Input: in}).WithContext(ctx), s)
	if rr.Code != http.StatusCreated {
		t.Fatalf("dynamic calculation could not be saved: %d %s", rr.Code, rr.Body.String())
	}
	var saved map[string]json.RawMessage
	if err = json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	var revisionID, selectedVersion string
	if err = json.Unmarshal(saved["revisionId"], &revisionID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT engine_version FROM ctc_revisions WHERE id=$1", revisionID).Scan(&selectedVersion); err != nil || selectedVersion != engine.RuleEngineVersion {
		t.Fatalf("selected engine version not persisted: %s %v", selectedVersion, err)
	}
	var amount string
	if err = pool.QueryRow(ctx, "SELECT monthly_amount::text FROM calculation_lines WHERE revision_id=$1 AND component_code='WELLNESS'", revisionID).Scan(&amount); err != nil || !strings.HasPrefix(amount, "100.") {
		t.Fatalf("dynamic component line not persisted: %s %v", amount, err)
	}
	derived := engine.ConvertConfiguration(v2)
	for i := range derived.Data.Rules.Components {
		if derived.Data.Rules.Components[i].Code == "BASIC" {
			derived.Data.Rules.Components[i].Rules[0].Expression = &engine.Expression{Op: "mul", Args: []engine.Expression{{Op: "ref", Ref: "input.gross"}, {Op: "number", Value: "0.2"}}}
		}
	}
	derived, err = a.insertConfig(ctx, derived, s.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	derivedInput := in
	derivedInput.Basic = ""
	rr = httptest.NewRecorder()
	a.saveCase(rr, ruleRequest(t, "/api/cases", calculationRequest{ConfigurationID: derived.ID, Input: derivedInput}).WithContext(ctx), s)
	if rr.Code != http.StatusCreated {
		t.Fatalf("derived Basic incorrectly requires manual input: %d %s", rr.Code, rr.Body.String())
	}
	var omittedBasic bool
	if err = pool.QueryRow(ctx, "SELECT basic IS NULL FROM ctc_revisions WHERE configuration_id=$1", derived.ID).Scan(&omittedBasic); err != nil || !omittedBasic {
		t.Fatalf("absent manual Basic was not preserved as absent: %v", err)
	}
	exec("UPDATE configurations SET status='published',published_at=now() WHERE id=$1", v2.ID)
	_, err = pool.Exec(ctx, "INSERT INTO group_configurations(configuration_id,group_code,name,display_order,definition) VALUES($1,'later','Late group',99,'{}')", v2.ID)
	if err == nil || !strings.Contains(err.Error(), "Published configuration versions are immutable") {
		t.Fatalf("published configuration accepted another group: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE configuration_rule_sets SET rules='{}' WHERE configuration_id=$1", v2.ID)
	if err == nil || !strings.Contains(err.Error(), "Historical records are immutable") {
		t.Fatalf("saved rules were mutable: %v", err)
	}
}
