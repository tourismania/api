ALTER TABLE "users"
    ADD COLUMN agency_id INT NULL REFERENCES agencies(id);

-- Existing rows (table predates this feature) get backfilled to the
-- seeded "ТУРИЗМАНИЯ" agency. Looked up by name rather than a hardcoded
-- DB-level DEFAULT: the id is only known after migration 013 inserted it.
UPDATE "users"
SET agency_id = (SELECT id FROM agencies WHERE name = 'ТУРИЗМАНИЯ')
WHERE agency_id IS NULL;

ALTER TABLE "users"
    ALTER COLUMN agency_id SET NOT NULL;
