DO $$
DECLARE tab text; col text;
BEGIN
    FOREACH tab IN ARRAY ARRAY['fy-sites','fy-blocks','fy-ox-users','fy-dues-definitions',
        'fy-accruals','fy-fiscal-periods','fy-ledger-accounts','fy-bank-statements',
        'fy-decisions','fy-debt-restructurings','fy-audit-logs','fy-idempotency-keys']
    LOOP
        col := CASE WHEN tab = 'fy-sites' THEN 'id' ELSE 'site_id' END;
        EXECUTE format('DROP POLICY IF EXISTS tenant_scope ON %I',tab);
        EXECUTE format('CREATE POLICY tenant_read ON %I FOR SELECT USING (app_is_bypass_rls() OR %s = app_get_site_id())',tab,col);
        EXECUTE format('CREATE POLICY tenant_write ON %I FOR ALL USING (app_is_bypass_rls() OR (%s = app_get_site_id() AND app_get_role() IN (''manager'',''accountant''))) WITH CHECK (app_is_bypass_rls() OR (%s = app_get_site_id() AND app_get_role() IN (''manager'',''accountant'')))',tab,col,col);
    END LOOP;
END $$;
CREATE POLICY audit_append ON "fy-audit-logs" FOR INSERT WITH CHECK (site_id = app_get_site_id() AND user_id = app_get_user_id());
CREATE POLICY idempotency_insert ON "fy-idempotency-keys" FOR INSERT WITH CHECK (site_id = app_get_site_id() AND user_id = app_get_user_id());
ALTER TABLE "fy-unit-occupancies" ENABLE ROW LEVEL SECURITY;
CREATE POLICY occupancy_scope ON "fy-unit-occupancies" FOR ALL
    USING (EXISTS (SELECT FROM "fy-units" u WHERE u.id=unit_id))
    WITH CHECK (app_get_role() IN ('manager','accountant','superadmin') AND EXISTS (SELECT FROM "fy-units" u WHERE u.id=unit_id));

-- Foreign keys enforce identity; these checks also enforce tenant ownership.
CREATE FUNCTION enforce_tenant_references() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF current_user != 'siteyonet_runtime' THEN RETURN NEW; END IF;
    IF TG_TABLE_NAME = 'fy-units' THEN
        IF NOT EXISTS (SELECT FROM "fy-blocks" WHERE id=NEW.block_id AND site_id=NEW.site_id) THEN RAISE EXCEPTION 'Foreign block'; END IF;
        IF NEW.owner_id IS NOT NULL AND NOT EXISTS (SELECT FROM "fy-ox-users" WHERE id=NEW.owner_id AND site_id=NEW.site_id) THEN RAISE EXCEPTION 'Foreign owner'; END IF;
        IF NEW.tenant_id IS NOT NULL AND NOT EXISTS (SELECT FROM "fy-ox-users" WHERE id=NEW.tenant_id AND site_id=NEW.site_id) THEN RAISE EXCEPTION 'Foreign tenant'; END IF;
    ELSIF TG_TABLE_NAME = 'fy-journal-lines' THEN
        IF NOT EXISTS (SELECT FROM "fy-ledger-accounts" a JOIN "fy-journal-entries" e ON e.site_id=a.site_id WHERE a.id=NEW.account_id AND e.id=NEW.journal_entry_id) THEN RAISE EXCEPTION 'Foreign ledger account'; END IF;
    ELSE
        IF NEW.unit_id IS NOT NULL AND NOT EXISTS (SELECT FROM "fy-units" WHERE id=NEW.unit_id AND site_id=NEW.site_id) THEN RAISE EXCEPTION 'Foreign unit'; END IF;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER unit_tenant BEFORE INSERT OR UPDATE ON "fy-units" FOR EACH ROW EXECUTE FUNCTION enforce_tenant_references();
CREATE TRIGGER payment_tenant BEFORE INSERT OR UPDATE ON "fy-payments" FOR EACH ROW EXECUTE FUNCTION enforce_tenant_references();
CREATE TRIGGER debt_tenant BEFORE INSERT OR UPDATE ON "fy-debts" FOR EACH ROW EXECUTE FUNCTION enforce_tenant_references();
CREATE TRIGGER maintenance_tenant BEFORE INSERT OR UPDATE ON "fy-maintenance-requests" FOR EACH ROW EXECUTE FUNCTION enforce_tenant_references();
CREATE TRIGGER journal_account_tenant BEFORE INSERT OR UPDATE ON "fy-journal-lines" FOR EACH ROW EXECUTE FUNCTION enforce_tenant_references();
