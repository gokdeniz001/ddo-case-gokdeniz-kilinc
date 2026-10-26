CREATE FUNCTION app_has_unit_history(unit uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=public AS $$
    SELECT EXISTS (SELECT FROM "fy-unit-occupancies" WHERE unit_id=unit AND user_id=app_get_user_id());
$$;
CREATE POLICY historical_unit_read ON "fy-units" FOR SELECT USING (
    site_id=app_get_site_id() AND app_get_role() IN ('owner','tenant') AND app_has_unit_history(id)
);
