ALTER TABLE "users"
    ADD COLUMN agency_id INT NULL REFERENCES agencies(id);
