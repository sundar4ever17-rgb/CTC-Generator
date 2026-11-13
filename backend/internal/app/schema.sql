CREATE TABLE IF NOT EXISTS users (
 id text PRIMARY KEY, name text NOT NULL, email text UNIQUE NOT NULL, password_hash text NOT NULL,
 role text NOT NULL CHECK(role IN ('hr','cb','admin','auditor')), active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
 token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id), csrf_token text NOT NULL,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS configurations (
 id text PRIMARY KEY, name text NOT NULL, version integer GENERATED ALWAYS AS IDENTITY UNIQUE,
 source text NOT NULL CHECK(source IN ('urd-demo','client')), status text NOT NULL CHECK(status IN ('draft','published')),
 effective_from date, effective_to date, data jsonb NOT NULL, created_by text REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz,
 CHECK(effective_from IS NULL OR effective_to IS NULL OR effective_from<=effective_to)
);
CREATE TABLE IF NOT EXISTS salary_components (code text PRIMARY KEY, name text NOT NULL);
CREATE TABLE IF NOT EXISTS qualifications (code text PRIMARY KEY, name text NOT NULL);
INSERT INTO qualifications(code,name) VALUES ('NONE','None'),('CA','Chartered Accountant'),('CMA','Cost and Management Accountant'),('CS','Company Secretary') ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS component_configurations (
 configuration_id text NOT NULL REFERENCES configurations(id), component_code text NOT NULL REFERENCES salary_components(code),
 group_code text NOT NULL CHECK(group_code IN ('inclusion','exclusion','statutory','policy','variable')),
 display_order integer NOT NULL, PRIMARY KEY(configuration_id,component_code)
);
CREATE TABLE IF NOT EXISTS grade_salary_configurations (
 configuration_id text NOT NULL REFERENCES configurations(id), grade_id text NOT NULL, name text NOT NULL,
 basic_min numeric(24,8), basic_max numeric(24,8), conveyance numeric(24,8), conveyance_cap numeric(24,8), basic_selection_method text NOT NULL,
 PRIMARY KEY(configuration_id,grade_id),CHECK(basic_min IS NULL OR basic_max IS NULL OR basic_min<=basic_max)
);
CREATE TABLE IF NOT EXISTS location_configurations (
 configuration_id text NOT NULL REFERENCES configurations(id), location_id text NOT NULL, name text NOT NULL,
 PRIMARY KEY(configuration_id,location_id)
);
CREATE TABLE IF NOT EXISTS professional_allowance_configurations (
 configuration_id text NOT NULL, grade_id text NOT NULL, qualification_code text NOT NULL REFERENCES qualifications(code),
 amount numeric(24,8) NOT NULL CHECK(amount>=0), PRIMARY KEY(configuration_id,grade_id,qualification_code),
 FOREIGN KEY(configuration_id,grade_id) REFERENCES grade_salary_configurations(configuration_id,grade_id)
);
CREATE TABLE IF NOT EXISTS core_calculation_parameters (
 configuration_id text PRIMARY KEY REFERENCES configurations(id),
 pf_basic_threshold numeric(24,8) NOT NULL,pf_inclusion_share numeric(24,8) NOT NULL,pf_basis_cap numeric(24,8) NOT NULL,pf_rate numeric(24,8) NOT NULL,
 nps_old_rate numeric(24,8) NOT NULL,nps_new_rate numeric(24,8) NOT NULL,esi_rate numeric(24,8) NOT NULL,esi_ceiling numeric(24,8),
 esi_eligibility_basis text,esi_contribution_basis text,esi_ceiling_inclusive boolean,gratuity_rate numeric(24,8) NOT NULL,rounding_policy jsonb NOT NULL
);
CREATE TABLE IF NOT EXISTS benefit_configurations (
 id text PRIMARY KEY,configuration_id text NOT NULL REFERENCES configurations(id), component_code text NOT NULL,
 grade_id text,location_id text,rule_kind text NOT NULL CHECK(rule_kind IN ('pending','none','fixed','percent','schedule')),
 amount numeric(24,8),rate numeric(24,8),basis_code text,basis_period text,cap numeric(24,8),period text,conditions jsonb NOT NULL,notes text NOT NULL,
 FOREIGN KEY(configuration_id,component_code) REFERENCES component_configurations(configuration_id,component_code),
 FOREIGN KEY(configuration_id,grade_id) REFERENCES grade_salary_configurations(configuration_id,grade_id),
 FOREIGN KEY(configuration_id,location_id) REFERENCES location_configurations(configuration_id,location_id)
);
CREATE TABLE IF NOT EXISTS benefit_schedule_items (
 benefit_id text NOT NULL REFERENCES benefit_configurations(id),year integer NOT NULL CHECK(year>0),
 amount numeric(24,8) NOT NULL CHECK(amount>=0),PRIMARY KEY(benefit_id,year)
);
CREATE TABLE IF NOT EXISTS candidates (
 id text PRIMARY KEY,candidate_reference text NOT NULL,name text NOT NULL,profile_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS ctc_cases (
 id text PRIMARY KEY,candidate_id text NOT NULL REFERENCES candidates(id),owner_user_id text NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS ctc_revisions (
 id text PRIMARY KEY,case_id text NOT NULL REFERENCES ctc_cases(id),revision_number integer NOT NULL,
 configuration_id text NOT NULL REFERENCES configurations(id),engine_version text NOT NULL,
 grade_id text NOT NULL,location_id text NOT NULL,qualification_code text NOT NULL REFERENCES qualifications(code),
 gross numeric(24,8) NOT NULL,basic numeric(24,8) NOT NULL,tax_regime text NOT NULL CHECK(tax_regime IN ('new','old')),
 nps_opt_in boolean NOT NULL,input_snapshot jsonb NOT NULL,result_snapshot jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('complete','incomplete')),created_by text NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(case_id,revision_number),UNIQUE(id,configuration_id),
 FOREIGN KEY(configuration_id,grade_id) REFERENCES grade_salary_configurations(configuration_id,grade_id),
 FOREIGN KEY(configuration_id,location_id) REFERENCES location_configurations(configuration_id,location_id)
);
CREATE TABLE IF NOT EXISTS calculation_lines (
 revision_id text NOT NULL,configuration_id text NOT NULL,component_code text NOT NULL,
 monthly_amount numeric(24,8),annual_amount numeric(24,8),amount numeric(24,8),amount_period text,
 applicability_status text NOT NULL,formula_trace text NOT NULL,
 PRIMARY KEY(revision_id,component_code),
 FOREIGN KEY(revision_id,configuration_id) REFERENCES ctc_revisions(id,configuration_id),
 FOREIGN KEY(configuration_id,component_code) REFERENCES component_configurations(configuration_id,component_code)
);
CREATE TABLE IF NOT EXISTS document_template_versions (
 id text PRIMARY KEY,configuration_id text NOT NULL REFERENCES configurations(id),document_type text NOT NULL CHECK(document_type IN ('ctc','offer')),
 template_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(configuration_id,document_type)
);
CREATE TABLE IF NOT EXISTS generated_documents (
 id text PRIMARY KEY,revision_id text NOT NULL REFERENCES ctc_revisions(id),template_version_id text NOT NULL REFERENCES document_template_versions(id),
 document_type text NOT NULL CHECK(document_type IN ('ctc','offer')),mode text NOT NULL CHECK(mode IN ('draft','final')),
 content bytea NOT NULL,checksum text NOT NULL,generated_by text NOT NULL REFERENCES users(id),generated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(revision_id,document_type,mode)
);
CREATE TABLE IF NOT EXISTS audit_events (
 id text PRIMARY KEY,actor_id text REFERENCES users(id),action text NOT NULL,entity_type text NOT NULL,entity_id text NOT NULL,
 details jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_cases_owner ON ctc_cases(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_revisions_case ON ctc_revisions(case_id,revision_number DESC);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
CREATE OR REPLACE FUNCTION prevent_config_rewrite() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR NEW.data IS DISTINCT FROM OLD.data OR NEW.source IS DISTINCT FROM OLD.source OR NEW.name IS DISTINCT FROM OLD.name
 OR NEW.effective_from IS DISTINCT FROM OLD.effective_from OR NEW.effective_to IS DISTINCT FROM OLD.effective_to
 OR (OLD.status='published' AND NEW.status<>OLD.status) THEN RAISE EXCEPTION 'Configuration versions are immutable; create a new version'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS configuration_immutable ON configurations;
CREATE TRIGGER configuration_immutable BEFORE UPDATE OR DELETE ON configurations FOR EACH ROW EXECUTE FUNCTION prevent_config_rewrite();
CREATE OR REPLACE FUNCTION prevent_history_rewrite() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Historical records are immutable'; END $$;
DROP TRIGGER IF EXISTS revision_immutable ON ctc_revisions;
CREATE TRIGGER revision_immutable BEFORE UPDATE OR DELETE ON ctc_revisions FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
DROP TRIGGER IF EXISTS audit_immutable ON audit_events;
CREATE TRIGGER audit_immutable BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
