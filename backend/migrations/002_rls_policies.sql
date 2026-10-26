-- PostgreSQL 16 Migration: Row Level Security (RLS) Policies
-- Task 3 (T-3): Multi-tenant isolation at the database level

-- Helper function to check if RLS bypass is requested (e.g. system seed or migrations)
CREATE OR REPLACE FUNCTION app_is_bypass_rls() RETURNS BOOLEAN AS $$
BEGIN
    RETURN current_setting('app.bypass_rls', true) = 'true' 
        OR current_setting('app.current_user_role', true) = 'superadmin';
END;
$$ LANGUAGE plpgsql STABLE;

-- Helper function to get current site id
CREATE OR REPLACE FUNCTION app_get_site_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_site_id', true), '')::UUID;
EXCEPTION WHEN OTHERS THEN
    RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

-- Helper function to get current user id
CREATE OR REPLACE FUNCTION app_get_user_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_user_id', true), '')::UUID;
EXCEPTION WHEN OTHERS THEN
    RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

-- Helper function to get current role
CREATE OR REPLACE FUNCTION app_get_role() RETURNS TEXT AS $$
BEGIN
    RETURN current_setting('app.current_user_role', true);
END;
$$ LANGUAGE plpgsql STABLE;

---------------------------------------------------------
-- 1. Enable RLS on Tenant-specific tables
---------------------------------------------------------
ALTER TABLE "fy-units" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-debts" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-payments" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-expenses" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-journal-entries" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-announcements" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-maintenance-requests" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "fy-clearance-certificates" ENABLE ROW LEVEL SECURITY;

---------------------------------------------------------
-- 2. Policies for "fy-units"
---------------------------------------------------------
CREATE POLICY units_site_manager_all ON "fy-units"
    FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')));

CREATE POLICY units_auditor_select ON "fy-units"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() = 'auditor'));

CREATE POLICY units_resident_select ON "fy-units"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND (owner_id = app_get_user_id() OR tenant_id = app_get_user_id())));

---------------------------------------------------------
-- 3. Policies for "fy-debts" (Resident can ONLY see their own debts)
---------------------------------------------------------
CREATE POLICY debts_manager_all ON "fy-debts"
    FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')));

CREATE POLICY debts_auditor_select ON "fy-debts"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() = 'auditor'));

CREATE POLICY debts_resident_select ON "fy-debts"
    FOR SELECT
    USING (app_is_bypass_rls() OR (
        site_id = app_get_site_id() 
        AND app_get_role() IN ('owner', 'tenant') 
        AND (user_id = app_get_user_id() OR unit_id IN (
            SELECT id FROM "fy-units" WHERE owner_id = app_get_user_id() OR tenant_id = app_get_user_id()
        ))
    ));

---------------------------------------------------------
-- 4. Policies for "fy-payments"
---------------------------------------------------------
CREATE POLICY payments_manager_all ON "fy-payments"
    FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')));

CREATE POLICY payments_auditor_select ON "fy-payments"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() = 'auditor'));

CREATE POLICY payments_resident_select ON "fy-payments"
    FOR SELECT
    USING (app_is_bypass_rls() OR (
        site_id = app_get_site_id() 
        AND app_get_role() IN ('owner', 'tenant') 
        AND (user_id = app_get_user_id() OR unit_id IN (
            SELECT id FROM "fy-units" WHERE owner_id = app_get_user_id() OR tenant_id = app_get_user_id()
        ))
    ));

---------------------------------------------------------
-- 5. Policies for "fy-expenses" (Auditors read-only, Residents cannot mutate)
---------------------------------------------------------
CREATE POLICY expenses_manager_all ON "fy-expenses"
    FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')));

CREATE POLICY expenses_auditor_resident_select ON "fy-expenses"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('auditor', 'owner', 'tenant')));

---------------------------------------------------------
-- 6. Policies for "fy-journal-entries" (Auditor read-only)
---------------------------------------------------------
CREATE POLICY journal_manager_all ON "fy-journal-entries"
    FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')));

CREATE POLICY journal_auditor_select ON "fy-journal-entries"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() = 'auditor'));

---------------------------------------------------------
-- 7. Policies for "fy-clearance-certificates"
---------------------------------------------------------
CREATE POLICY cert_manager_all ON "fy-clearance-certificates"
    FOR ALL
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')))
    WITH CHECK (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() IN ('manager', 'accountant')));

CREATE POLICY cert_auditor_select ON "fy-clearance-certificates"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND app_get_role() = 'auditor'));

CREATE POLICY cert_resident_select ON "fy-clearance-certificates"
    FOR SELECT
    USING (app_is_bypass_rls() OR (site_id = app_get_site_id() AND user_id = app_get_user_id()));
