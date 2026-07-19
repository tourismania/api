CREATE TABLE offers (
    id          SERIAL       PRIMARY KEY,
    uuid        UUID         NOT NULL UNIQUE,
    title       VARCHAR(200) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    agency_id   INT          NOT NULL REFERENCES agencies(id),
    created_by  INT          NOT NULL REFERENCES users(id),
    status      VARCHAR(20)  NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMP    NOT NULL,
    updated_at  TIMESTAMP    NOT NULL,
    deleted_at  TIMESTAMP    NULL
);

CREATE INDEX offers_agency_id_idx ON offers (agency_id);
CREATE INDEX offers_status_idx    ON offers (status);
CREATE INDEX offers_active_idx    ON offers (id) WHERE deleted_at IS NULL;
