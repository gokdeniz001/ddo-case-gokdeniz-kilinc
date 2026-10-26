\pset pager off
SELECT id AS site_id FROM "fy-sites" WHERE code='STE01' \gset
SELECT id AS unit_id FROM "fy-units" WHERE site_id=:'site_id' AND unit_number='5' LIMIT 1 \gset
BEGIN;
SET LOCAL ROLE siteyonet_runtime;
SELECT set_config('app.current_site_id',:'site_id',true),set_config('app.current_user_role','manager',true);
\echo KEYSET BASELINE - forced sequential scan, same data and query
SET LOCAL enable_indexscan=off;
SET LOCAL enable_indexonlyscan=off;
SET LOCAL enable_bitmapscan=off;
EXPLAIN (ANALYZE,BUFFERS) SELECT id,principal_cents,due_date FROM "fy-debts" WHERE site_id=:'site_id' ORDER BY due_date,id LIMIT 500;
\echo UNIT DEBT BASELINE - forced sequential scan
EXPLAIN (ANALYZE,BUFFERS) SELECT id,principal_cents FROM "fy-debts" WHERE unit_id=:'unit_id' AND status='UNPAID';
\echo KEYSET INDEX - normal planner
SET LOCAL enable_indexscan=on;
SET LOCAL enable_indexonlyscan=on;
SET LOCAL enable_bitmapscan=on;
EXPLAIN (ANALYZE,BUFFERS) SELECT id,principal_cents,due_date FROM "fy-debts" WHERE site_id=:'site_id' ORDER BY due_date,id LIMIT 500;
\echo UNIT DEBT INDEX - normal planner
EXPLAIN (ANALYZE,BUFFERS) SELECT id,principal_cents FROM "fy-debts" WHERE unit_id=:'unit_id' AND status='UNPAID';
ROLLBACK;
