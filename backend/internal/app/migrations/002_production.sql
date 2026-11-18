ALTER TABLE users ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN auth_version bigint NOT NULL DEFAULT 1;
ALTER TABLE sessions ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE sessions ADD COLUMN auth_version bigint NOT NULL DEFAULT 1;
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_cases_updated ON ctc_cases(updated_at DESC,id);
CREATE INDEX idx_cases_owner_updated ON ctc_cases(owner_user_id,updated_at DESC,id);
CREATE INDEX idx_audit_actor_created ON audit_events(actor_id,created_at DESC);
CREATE INDEX idx_candidates_reference ON candidates(candidate_reference);
CREATE TABLE auth_rate_limits (
 key_hash text PRIMARY KEY,
 attempts integer NOT NULL CHECK(attempts>0),
 window_end timestamptz NOT NULL
);
CREATE INDEX idx_auth_limits_expiry ON auth_rate_limits(window_end);
CREATE TRIGGER calculation_lines_immutable BEFORE UPDATE OR DELETE ON calculation_lines FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER template_immutable BEFORE UPDATE OR DELETE ON document_template_versions FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER documents_immutable BEFORE UPDATE OR DELETE ON generated_documents FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER component_config_immutable BEFORE UPDATE OR DELETE ON component_configurations FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER grades_immutable BEFORE UPDATE OR DELETE ON grade_salary_configurations FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER locations_immutable BEFORE UPDATE OR DELETE ON location_configurations FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER professional_config_immutable BEFORE UPDATE OR DELETE ON professional_allowance_configurations FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER parameters_immutable BEFORE UPDATE OR DELETE ON core_calculation_parameters FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER benefits_immutable BEFORE UPDATE OR DELETE ON benefit_configurations FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER schedules_immutable BEFORE UPDATE OR DELETE ON benefit_schedule_items FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
