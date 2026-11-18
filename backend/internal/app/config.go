package app

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql
var schema string

func (a *App) initialize(ctx context.Context) error {
	conn, e := a.DB.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	if _, e = conn.Exec(ctx, "SELECT pg_advisory_lock(42900821)"); e != nil {
		return e
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(42900821)")
	if a.Demo {
		password := os.Getenv("DEMO_PASSWORD")
		if password == "" {
			password = "DemoCTC!2026"
		}
		if !validPassword(password) {
			return fmt.Errorf("DEMO_PASSWORD must contain 12-72 characters")
		}
		ph, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if e != nil {
			return e
		}
		for _, role := range []string{"cb", "hr", "admin", "auditor"} {
			_, e = conn.Exec(ctx, "INSERT INTO users(id,name,email,password_hash,role) VALUES($1,$2,$3,$4,$5) ON CONFLICT(email) DO NOTHING", ID(), map[string]string{"cb": "Compensation & Benefits", "hr": "HR Partner", "admin": "Demo Administrator", "auditor": "Audit Reviewer"}[role], role+"@demo.local", string(ph), role)
			if e != nil {
				return e
			}
		}
		var count int
		if e = conn.QueryRow(ctx, "SELECT count(*) FROM configurations WHERE source='urd-demo'").Scan(&count); e != nil {
			return e
		}
		if count == 0 {
			c := engine.DemoConfiguration()
			if _, e = a.insertConfig(ctx, c, ""); e != nil {
				return e
			}
		}
	}
	return nil
}

const configSelect = "SELECT id,name,version,source,status,COALESCE(effective_from::text,''),COALESCE(effective_to::text,''),data,created_at FROM configurations"

func scanConfig(row pgx.Row) (engine.Configuration, error) {
	var c engine.Configuration
	var b []byte
	var at time.Time
	e := row.Scan(&c.ID, &c.Name, &c.Version, &c.Source, &c.Status, &c.EffectiveFrom, &c.EffectiveTo, &b, &at)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c.Data)
	c.CreatedAt = at.Format(time.RFC3339)
	return c, e
}
func (a *App) config(ctx context.Context, id string) (engine.Configuration, error) {
	return scanConfig(a.DB.QueryRow(ctx, configSelect+" WHERE id=$1", id))
}
func (a *App) configs(ctx context.Context) ([]engine.Configuration, error) {
	return a.configPage(ctx, 100, 0)
}
func (a *App) configPage(ctx context.Context, limit, offset int) ([]engine.Configuration, error) {
	rows, e := a.DB.Query(ctx, configSelect+" WHERE ($1 OR source='client') ORDER BY version DESC LIMIT $2 OFFSET $3", a.Demo, limit, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []engine.Configuration{}
	for rows.Next() {
		c, e := scanConfig(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (a *App) insertConfig(ctx context.Context, c engine.Configuration, actor string) (engine.Configuration, error) {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return c, e
	}
	defer tx.Rollback(ctx)
	c.ID = ID()
	b, _ := json.Marshal(c.Data)
	var at time.Time
	e = tx.QueryRow(ctx, `INSERT INTO configurations(id,name,source,status,effective_from,effective_to,data,created_by,published_at) VALUES($1,$2,$3,$4,$5::date,$6::date,$7,$8,CASE WHEN $4='published' THEN now() ELSE NULL END) RETURNING version,created_at`, c.ID, c.Name, c.Source, c.Status, nullable(c.EffectiveFrom), nullable(c.EffectiveTo), b, nullable(actor)).Scan(&c.Version, &at)
	if e != nil {
		return c, e
	}
	c.CreatedAt = at.Format(time.RFC3339)
	for i, component := range engine.Components {
		group := component.Group
		if component.Code == "ANNUAL_SPECIAL" && c.Data.AnnualSpecialGroup == "policy" {
			group = "policy"
		}
		if _, e = tx.Exec(ctx, "INSERT INTO component_configurations(configuration_id,component_code,group_code,display_order) VALUES($1,$2,$3,$4)", c.ID, component.Code, group, i); e != nil {
			return c, e
		}
	}
	for _, g := range c.Data.Grades {
		_, e = tx.Exec(ctx, "INSERT INTO grade_salary_configurations(configuration_id,grade_id,name,basic_min,basic_max,conveyance,conveyance_cap,basic_selection_method) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", c.ID, g.ID, g.Name, g.BasicMin, g.BasicMax, g.Conveyance, g.ConveyanceCap, c.Data.BasicMethod)
		if e != nil {
			return c, e
		}
		for q, amount := range g.ProfessionalAllowance {
			if _, e = tx.Exec(ctx, "INSERT INTO professional_allowance_configurations(configuration_id,grade_id,qualification_code,amount) VALUES($1,$2,$3,$4)", c.ID, g.ID, q, amount); e != nil {
				return c, e
			}
		}
	}
	for _, l := range c.Data.Locations {
		if _, e = tx.Exec(ctx, "INSERT INTO location_configurations(configuration_id,location_id,name) VALUES($1,$2,$3)", c.ID, l.ID, l.Name); e != nil {
			return c, e
		}
	}
	d := c.Data
	rounding, _ := json.Marshal(d.Rounding)
	_, e = tx.Exec(ctx, `INSERT INTO core_calculation_parameters(configuration_id,pf_basic_threshold,pf_inclusion_share,pf_basis_cap,pf_rate,nps_old_rate,nps_new_rate,esi_rate,esi_ceiling,esi_eligibility_basis,esi_contribution_basis,gratuity_rate,rounding_policy,esi_ceiling_inclusive) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, c.ID, d.PF.BasicThreshold, d.PF.InclusionShare, d.PF.BasisCap, d.PF.Rate, d.NPS.OldRate, d.NPS.NewRate, d.ESI.Rate, d.ESI.Ceiling, nullable(d.ESI.EligibilityBasis), nullable(d.ESI.ContributionBasis), d.GratuityRate, rounding, d.ESI.CeilingInclusive)
	if e != nil {
		return c, e
	}
	for _, r := range d.Benefits {
		id := ID()
		conditions, _ := json.Marshal(r.Conditions)
		_, e = tx.Exec(ctx, `INSERT INTO benefit_configurations(id,configuration_id,component_code,grade_id,location_id,rule_kind,amount,rate,basis_code,cap,period,conditions,notes,basis_period) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, id, c.ID, r.Component, nullable(r.GradeID), nullable(r.LocationID), r.Kind, decimalNullable(r.Amount), decimalNullable(r.Rate), nullable(r.Basis), decimalNullable(r.Cap), nullable(r.Period), conditions, r.Notes, nullable(r.BasisPeriod))
		if e != nil {
			return c, e
		}
		for _, item := range r.Schedule {
			if _, e = tx.Exec(ctx, "INSERT INTO benefit_schedule_items(benefit_id,year,amount) VALUES($1,$2,$3)", id, item.Year, item.Amount); e != nil {
				return c, e
			}
		}
	}
	template, _ := json.Marshal(d.OfferTemplate)
	for _, kind := range []string{"ctc", "offer"} {
		if _, e = tx.Exec(ctx, "INSERT INTO document_template_versions(id,configuration_id,document_type,template_snapshot) VALUES($1,$2,$3,$4)", ID(), c.ID, kind, template); e != nil {
			return c, e
		}
	}
	details, _ := json.Marshal(map[string]any{"name": c.Name, "version": c.Version, "source": c.Source, "status": c.Status})
	_, e = tx.Exec(ctx, "INSERT INTO audit_events(id,actor_id,action,entity_type,entity_id,details) VALUES($1,$2,'configuration.create','configuration',$3,$4)", ID(), nullable(actor), c.ID, details)
	if e != nil {
		return c, e
	}
	e = tx.Commit(ctx)
	return c, e
}
func (a *App) listConfigs(w http.ResponseWriter, r *http.Request, s session) {
	limit, offset, ok := pagination(w, r)
	if !ok {
		return
	}
	items, e := a.configPage(r.Context(), limit, offset)
	if e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, items)
}
func (a *App) getConfig(w http.ResponseWriter, r *http.Request, s session) {
	c, e := a.config(r.Context(), r.PathValue("id"))
	if errors.Is(e, pgx.ErrNoRows) || (e == nil && !a.Demo && c.Source != "client") {
		fail(w, 404, "Configuration not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 200, c)
}
func (a *App) createConfig(w http.ResponseWriter, r *http.Request, s session) {
	var c engine.Configuration
	if !readJSON(w, r, &c) {
		return
	}
	c.Status = "draft"
	if !a.Demo && c.Source != "client" {
		fail(w, 400, "Production configurations must use client-approved data")
		return
	}
	if errs := engine.ValidateConfig(c); len(errs) > 0 {
		fail(w, 400, strings.Join(errs, "; "))
		return
	}
	c, e := a.insertConfig(r.Context(), c, s.User.ID)
	if e != nil {
		a.dbError(w, e)
		return
	}
	JSON(w, 201, c)
}
func (a *App) publishConfig(w http.ResponseWriter, r *http.Request, s session) {
	id := r.PathValue("id")
	c, e := a.config(r.Context(), id)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "Configuration not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	if errs := engine.ValidateConfig(c); len(errs) > 0 {
		fail(w, 400, strings.Join(errs, "; "))
		return
	}
	if !a.Demo {
		if errs := engine.ValidatePublication(c); len(errs) > 0 {
			fail(w, 422, strings.Join(errs, "; "))
			return
		}
	}
	if c.Status != "published" {
		tx, e := a.DB.Begin(r.Context())
		if e != nil {
			a.dbError(w, e)
			return
		}
		defer tx.Rollback(r.Context())
		tag, e := tx.Exec(r.Context(), "UPDATE configurations SET status='published',published_at=now() WHERE id=$1 AND status='draft'", id)
		if e == nil && tag.RowsAffected() > 0 {
			e = auditTx(r.Context(), tx, s.User.ID, "configuration.publish", "configuration", id, map[string]any{"version": c.Version, "source": c.Source})
		}
		if e == nil {
			e = tx.Commit(r.Context())
		}
		if e != nil {
			a.dbError(w, e)
			return
		}
	}
	c.Status = "published"
	JSON(w, 200, c)
}

type calculationRequest struct {
	ConfigurationID    string       `json:"configurationId"`
	Input              engine.Input `json:"input"`
	CaseID             string       `json:"caseId,omitempty"`
	ExpectedRevisionID string       `json:"expectedRevisionId,omitempty"`
}

func (a *App) calculate(w http.ResponseWriter, r *http.Request, s session) {
	var in calculationRequest
	if !readJSON(w, r, &in) {
		return
	}
	c, e := a.config(r.Context(), in.ConfigurationID)
	if errors.Is(e, pgx.ErrNoRows) {
		fail(w, 404, "Configuration not found")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	if !a.allowCalculation(w, c, in.Input, s.User) {
		return
	}
	result := engine.Calculate(c, in.Input)
	JSON(w, 200, result)
}

func (a *App) allowCalculation(w http.ResponseWriter, c engine.Configuration, in engine.Input, u User) bool {
	if !a.Demo {
		if c.Source != "client" {
			fail(w, 422, "Demo configuration cannot be used in production")
			return false
		}
		if in.CalculationDate == "" {
			fail(w, 400, "Calculation effective date is required")
			return false
		}
		if u.Role == "hr" && c.Status != "published" {
			fail(w, 403, "HR calculations require published configuration")
			return false
		}
	}
	return true
}
