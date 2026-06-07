CREATE TABLE cities (
    id           serial       PRIMARY KEY,
    name         varchar(100) NOT NULL,
    state        varchar(100),
    timezone     varchar(50),
    country_iso2 char(2)      NOT NULL REFERENCES countries(iso2)
);

CREATE INDEX cities_name_trgm_idx
    ON cities USING gin (lower(name) gin_trgm_ops);

