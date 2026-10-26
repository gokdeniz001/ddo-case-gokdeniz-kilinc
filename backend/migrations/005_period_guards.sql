CREATE FUNCTION enforce_open_period() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE period uuid; state text;
BEGIN
    IF current_user != 'siteyonet_runtime' THEN RETURN NEW; END IF;
    IF TG_TABLE_NAME = 'fy-journal-lines' THEN
        SELECT period_id INTO period FROM "fy-journal-entries" WHERE id = NEW.journal_entry_id;
    ELSE
        period := NEW.period_id;
    END IF;
    IF period IS NULL THEN RAISE EXCEPTION 'An open fiscal period is required'; END IF;
    SELECT status INTO state FROM "fy-fiscal-periods" WHERE id = period FOR SHARE;
    IF state IS DISTINCT FROM 'OPEN' THEN RAISE EXCEPTION 'Fiscal period is closed or inaccessible'; END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER open_period_entry BEFORE INSERT OR UPDATE ON "fy-journal-entries" FOR EACH ROW EXECUTE FUNCTION enforce_open_period();
CREATE TRIGGER open_period_line BEFORE INSERT OR UPDATE ON "fy-journal-lines" FOR EACH ROW EXECUTE FUNCTION enforce_open_period();
CREATE TRIGGER open_period_expense BEFORE INSERT OR UPDATE ON "fy-expenses" FOR EACH ROW EXECUTE FUNCTION enforce_open_period();
CREATE TRIGGER open_period_accrual BEFORE INSERT OR UPDATE ON "fy-accruals" FOR EACH ROW EXECUTE FUNCTION enforce_open_period();
CREATE UNIQUE INDEX one_journal_reversal ON "fy-journal-entries"(reversed_entry_id) WHERE reversed_entry_id IS NOT NULL;
