-- Run as the database/schema owner after every migration, against the dedicated
-- application database. Example: psql service=ctc -v app_role=ctc_runtime -f ...
-- The runtime login must not own the database/schema/tables or belong to the
-- owner role, and must have NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION.
\set ON_ERROR_STOP on
BEGIN;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM :"app_role";
GRANT USAGE ON SCHEMA public TO :"app_role";
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM :"app_role";
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM :"app_role";
GRANT SELECT ON schema_migrations, salary_components, qualifications TO :"app_role";
GRANT SELECT, UPDATE ON application_settings TO :"app_role";
GRANT SELECT, INSERT, UPDATE ON users, candidates, ctc_cases TO :"app_role";
GRANT SELECT, INSERT, UPDATE, DELETE ON sessions, auth_rate_limits TO :"app_role";
GRANT SELECT, INSERT ON configurations TO :"app_role";
GRANT UPDATE(status, published_at) ON configurations TO :"app_role";
GRANT SELECT, INSERT ON
    component_configurations, grade_salary_configurations,
    location_configurations, professional_allowance_configurations,
    core_calculation_parameters, benefit_configurations, benefit_schedule_items,
    ctc_revisions, calculation_lines, document_template_versions,
    generated_documents, audit_events
TO :"app_role";
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO :"app_role";
COMMIT;
