-- HTTP requests explicitly SET LOCAL ROLE to this non-owner, non-superuser role.
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'siteyonet_runtime') THEN
        CREATE ROLE siteyonet_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;
GRANT siteyonet_runtime TO CURRENT_USER;
GRANT USAGE ON SCHEMA public TO siteyonet_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO siteyonet_runtime;
REVOKE UPDATE, DELETE ON "fy-audit-logs" FROM siteyonet_runtime;

-- Complete the policies missing from the initial migration.
DO $$
DECLARE tab text;
BEGIN
    FOREACH tab IN ARRAY ARRAY['fy-sites','fy-blocks','fy-ox-users','fy-dues-definitions',
        'fy-accruals','fy-fiscal-periods','fy-ledger-accounts','fy-bank-statements',
        'fy-decisions','fy-debt-restructurings','fy-audit-logs','fy-idempotency-keys']
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',tab);
        EXECUTE format('CREATE POLICY tenant_scope ON %I FOR SELECT USING (app_is_bypass_rls() OR %s = app_get_site_id())',tab,
            CASE WHEN tab = 'fy-sites' THEN 'id' ELSE 'site_id' END);
    END LOOP;
END $$;

CREATE POLICY announcements_read ON "fy-announcements" FOR SELECT
    USING (app_is_bypass_rls() OR site_id = app_get_site_id());
CREATE POLICY announcements_write ON "fy-announcements" FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager','accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager','accountant')));
CREATE POLICY maintenance_scope ON "fy-maintenance-requests" FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND
        (app_get_role() IN ('manager','accountant','auditor') OR user_id = app_get_user_id())));
CREATE POLICY maintenance_insert ON "fy-maintenance-requests" FOR INSERT
    WITH CHECK (site_id = app_get_site_id() AND app_get_role() != 'auditor' AND
        (app_get_role() IN ('manager','accountant','superadmin') OR user_id = app_get_user_id()));
CREATE POLICY maintenance_update ON "fy-maintenance-requests" FOR UPDATE
    USING (site_id = app_get_site_id() AND app_get_role() IN ('manager','accountant','superadmin'));

ALTER TABLE "fy-journal-lines" ENABLE ROW LEVEL SECURITY;
CREATE POLICY journal_lines_scope ON "fy-journal-lines" FOR ALL
    USING (EXISTS (SELECT 1 FROM "fy-journal-entries" e WHERE e.id = journal_entry_id))
    WITH CHECK (app_get_role() IN ('manager','accountant','superadmin') AND
        EXISTS (SELECT 1 FROM "fy-journal-entries" e WHERE e.id = journal_entry_id));
ALTER TABLE "fy-payment-allocations" ENABLE ROW LEVEL SECURITY;
CREATE POLICY allocation_scope ON "fy-payment-allocations" FOR ALL
    USING (EXISTS (SELECT 1 FROM "fy-payments" p WHERE p.id = payment_id))
    WITH CHECK (app_get_role() IN ('manager','accountant','superadmin') AND
        EXISTS (SELECT 1 FROM "fy-payments" p WHERE p.id = payment_id));
