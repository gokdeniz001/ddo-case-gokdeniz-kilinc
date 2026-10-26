INSERT INTO "fy-user-sites" (user_id,site_id,role)
SELECT id,site_id,role FROM "fy-ox-users" WHERE site_id IS NOT NULL AND role != 'superadmin'
ON CONFLICT DO NOTHING;
ALTER TABLE "fy-user-sites" ENABLE ROW LEVEL SECURITY;
CREATE POLICY own_memberships ON "fy-user-sites" FOR SELECT USING (user_id=app_get_user_id() OR app_is_bypass_rls());
CREATE POLICY member_sites ON "fy-sites" FOR SELECT USING (EXISTS (
    SELECT FROM "fy-user-sites" m WHERE m.user_id=app_get_user_id() AND m.site_id="fy-sites".id
));
