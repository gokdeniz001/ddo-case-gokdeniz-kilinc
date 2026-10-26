CREATE TABLE "fy-budgets" (
    site_id uuid NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    category text NOT NULL,
    amount_cents bigint NOT NULL CHECK (amount_cents >= 0),
    PRIMARY KEY(site_id,category)
);
GRANT SELECT,INSERT,UPDATE ON "fy-budgets" TO siteyonet_runtime;
ALTER TABLE "fy-budgets" ENABLE ROW LEVEL SECURITY;
CREATE POLICY budget_read ON "fy-budgets" FOR SELECT USING (site_id=app_get_site_id() OR app_is_bypass_rls());
CREATE POLICY budget_write ON "fy-budgets" FOR ALL
    USING (site_id=app_get_site_id() AND app_get_role() IN ('manager','accountant','superadmin'))
    WITH CHECK (site_id=app_get_site_id() AND app_get_role() IN ('manager','accountant','superadmin'));
INSERT INTO "fy-ledger-accounts" (site_id,code,name,account_type)
SELECT id,'642','Gecikme Zammı Gelirleri','REVENUE' FROM "fy-sites" ON CONFLICT DO NOTHING;
