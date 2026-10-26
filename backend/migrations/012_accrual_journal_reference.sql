ALTER TABLE "fy-accruals" ADD COLUMN journal_entry_id uuid REFERENCES "fy-journal-entries"(id) ON DELETE SET NULL;
