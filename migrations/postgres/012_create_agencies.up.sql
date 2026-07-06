CREATE TABLE agencies (
    id         SERIAL       PRIMARY KEY,
    uuid       UUID         NOT NULL UNIQUE,
    name       VARCHAR(200) NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at TIMESTAMP    NOT NULL,
    deleted_at TIMESTAMP    NULL
);

CREATE INDEX agencies_active_idx ON agencies (id) WHERE deleted_at IS NULL;
