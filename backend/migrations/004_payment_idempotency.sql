ALTER TABLE "fy-payments" DROP CONSTRAINT IF EXISTS "fy-payments_idempotency_key_key";
UPDATE "fy-payments" SET idempotency_key = NULL WHERE idempotency_key = '';
CREATE UNIQUE INDEX payments_site_idempotency ON "fy-payments" (site_id, idempotency_key);
