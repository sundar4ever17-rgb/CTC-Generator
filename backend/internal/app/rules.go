package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"ctc/internal/engine"
	"github.com/jackc/pgx/v5"
)

// The canonical policy is configurations.data. Projection rows are produced
// only from that same validated snapshot and committed in the same transaction.
func insertRuleProjections(ctx context.Context, tx pgx.Tx, c engine.Configuration) error {
	groups := []engine.GroupDefinition{
		{Code: "inclusion", Name: "Inclusion wages", InGross: true},
		{Code: "exclusion", Name: "Exclusion wages", InGross: true},
		{Code: "statutory", Name: "Statutory payments"},
		{Code: "policy", Name: "Policy-based payments"},
		{Code: "variable", Name: "Variable pay"},
	}
	components := make([]engine.ComponentDefinition, 0, len(engine.Components))
	if c.Data.Rules != nil {
		groups = c.Data.Rules.Groups
		components = c.Data.Rules.Components
	} else {
		for _, item := range engine.Components {
			group := item.Group
			if item.Code == "ANNUAL_SPECIAL" && c.Data.AnnualSpecialGroup == "policy" {
				group = "policy"
			}
			components = append(components, engine.ComponentDefinition{Code: item.Code, Name: item.Name, Group: group})
		}
	}
	for order, group := range groups {
		definition, err := json.Marshal(group)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO group_configurations(configuration_id,group_code,name,display_order,definition) VALUES($1,$2,$3,$4,$5)", c.ID, group.Code, group.Name, order, definition); err != nil {
			return err
		}
	}
	for order, component := range components {
		// Global codes are stable identities. Labels and rules are versioned in
		// the projection and canonical snapshot; an existing code is not renamed.
		if _, err := tx.Exec(ctx, "INSERT INTO salary_components(code,name) VALUES($1,$2) ON CONFLICT DO NOTHING", component.Code, component.Name); err != nil {
			return err
		}
		definition, err := json.Marshal(component)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO component_configurations(configuration_id,component_code,group_code,display_order,definition) VALUES($1,$2,$3,$4,$5)", c.ID, component.Code, component.Group, order, definition); err != nil {
			return err
		}
	}
	if c.Data.Rules != nil {
		rules, err := json.Marshal(c.Data.Rules)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO configuration_rule_sets(configuration_id,rules) VALUES($1,$2)", c.ID, rules)
		return err
	}
	return nil
}

// Conversion deliberately does not save or publish a policy. The editor can
// inspect, preview and save the returned draft as a separate version.
func (a *App) convertConfig(w http.ResponseWriter, r *http.Request, s session) {
	var c engine.Configuration
	if !readJSON(w, r, &c) {
		return
	}
	if !a.Demo && c.Source != "client" {
		fail(w, http.StatusBadRequest, "Production configurations must use client-approved data")
		return
	}
	if errs := engine.ValidateConfig(c); len(errs) > 0 {
		fail(w, http.StatusBadRequest, strings.Join(errs, "; "))
		return
	}
	converted := engine.ConvertConfiguration(c)
	if errs := engine.ValidateConfig(converted); len(errs) > 0 {
		fail(w, http.StatusBadRequest, strings.Join(errs, "; "))
		return
	}
	JSON(w, http.StatusOK, converted)
}

func (a *App) previewConfig(w http.ResponseWriter, r *http.Request, s session) {
	var in struct {
		Configuration engine.Configuration `json:"configuration"`
		Input         engine.Input         `json:"input"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	// A preview must never claim to be a published, finalizable calculation,
	// even if the caller copied a previously published configuration object.
	in.Configuration.ID = ""
	in.Configuration.Version = 0
	in.Configuration.Status = "draft"
	if errs := engine.ValidateConfig(in.Configuration); len(errs) > 0 {
		fail(w, http.StatusBadRequest, strings.Join(errs, "; "))
		return
	}
	if !a.allowCalculation(w, in.Configuration, in.Input, s.User) {
		return
	}
	JSON(w, http.StatusOK, engine.Calculate(in.Configuration, in.Input))
}
