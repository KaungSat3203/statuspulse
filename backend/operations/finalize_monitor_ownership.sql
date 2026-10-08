-- EXPLICIT OPERATOR STEP ONLY. Not part of automatically applied migrations.
-- Run only after reviewing and assigning every legacy monitor.
BEGIN;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM monitors WHERE user_id IS NULL) THEN
        RAISE EXCEPTION 'Unassigned monitors remain; complete explicit ownership backfill first';
    END IF;
END $$;
ALTER TABLE monitors ALTER COLUMN user_id SET NOT NULL;
COMMIT;
