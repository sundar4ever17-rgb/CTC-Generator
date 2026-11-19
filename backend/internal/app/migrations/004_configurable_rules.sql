-- Configuration JSON remains the authoritative snapshot. These immutable
-- projections enforce version-specific component and category membership.
CREATE TABLE group_configurations (
 configuration_id text NOT NULL REFERENCES configurations(id),
 group_code text NOT NULL,
 name text NOT NULL,
 display_order integer NOT NULL,
 definition jsonb NOT NULL,
 PRIMARY KEY(configuration_id,group_code)
);
INSERT INTO group_configurations(configuration_id,group_code,name,display_order,definition)
SELECT c.id,g.code,g.name,g.position,jsonb_build_object('code',g.code,'name',g.name,'inGross',g.in_gross)
FROM configurations c CROSS JOIN (VALUES
 ('inclusion','Inclusion wages',0,true),
 ('exclusion','Exclusion wages',1,true),
 ('statutory','Statutory payments',2,false),
 ('policy','Policy-based payments',3,false),
 ('variable','Variable pay',4,false)
) AS g(code,name,position,in_gross);

ALTER TABLE component_configurations DROP CONSTRAINT component_configurations_group_code_check;
ALTER TABLE component_configurations ADD CONSTRAINT component_group_configuration_fk
 FOREIGN KEY(configuration_id,group_code) REFERENCES group_configurations(configuration_id,group_code);
ALTER TABLE component_configurations ADD COLUMN definition jsonb;
-- Generic policies may derive Basic or omit it. Legacy validation still
-- requires the original manual Basic input; existing values stay unchanged.
ALTER TABLE ctc_revisions ALTER COLUMN basic DROP NOT NULL;

CREATE TABLE configuration_rule_sets (
 configuration_id text PRIMARY KEY REFERENCES configurations(id),
 rules jsonb NOT NULL CHECK(jsonb_typeof(rules)='object' AND rules ? 'version' AND rules->>'version'='2')
);

CREATE TRIGGER group_config_immutable BEFORE UPDATE OR DELETE ON group_configurations
 FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER rule_sets_immutable BEFORE UPDATE OR DELETE ON configuration_rule_sets
 FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();
CREATE TRIGGER salary_component_identity_immutable BEFORE UPDATE OR DELETE ON salary_components
 FOR EACH ROW EXECUTE FUNCTION prevent_history_rewrite();

-- An already published policy cannot acquire extra components, rules, groups,
-- grades or template rows. Locking the parent serializes this with publication.
CREATE FUNCTION require_draft_configuration_child() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parent_status text;
BEGIN
 SELECT status INTO parent_status FROM configurations WHERE id=NEW.configuration_id FOR SHARE;
 IF parent_status IS DISTINCT FROM 'draft' THEN
  RAISE EXCEPTION 'Published configuration versions are immutable; create a new version';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER group_config_insert_guard BEFORE INSERT ON group_configurations
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER rule_sets_insert_guard BEFORE INSERT ON configuration_rule_sets
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER component_config_insert_guard BEFORE INSERT ON component_configurations
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER grade_config_insert_guard BEFORE INSERT ON grade_salary_configurations
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER location_config_insert_guard BEFORE INSERT ON location_configurations
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER professional_config_insert_guard BEFORE INSERT ON professional_allowance_configurations
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER core_parameters_insert_guard BEFORE INSERT ON core_calculation_parameters
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER benefit_config_insert_guard BEFORE INSERT ON benefit_configurations
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();
CREATE TRIGGER template_insert_guard BEFORE INSERT ON document_template_versions
 FOR EACH ROW EXECUTE FUNCTION require_draft_configuration_child();

CREATE FUNCTION require_draft_benefit_schedule() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parent_status text;
BEGIN
 SELECT c.status INTO parent_status FROM configurations c
 JOIN benefit_configurations b ON b.configuration_id=c.id WHERE b.id=NEW.benefit_id FOR SHARE OF c;
 IF parent_status IS DISTINCT FROM 'draft' THEN
  RAISE EXCEPTION 'Published configuration versions are immutable; create a new version';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER benefit_schedule_insert_guard BEFORE INSERT ON benefit_schedule_items
 FOR EACH ROW EXECUTE FUNCTION require_draft_benefit_schedule();
