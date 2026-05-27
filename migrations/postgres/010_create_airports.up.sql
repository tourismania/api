CREATE TABLE airports (
    icao         char(4)      PRIMARY KEY,
    iata         char(3)      UNIQUE,
    name         varchar(200) NOT NULL,
    location     float8[],
    elevation_ft int,
    city_id      int          NOT NULL REFERENCES cities(id)
);

CREATE INDEX airports_name_trgm_idx
    ON airports USING gin (lower(unaccent(name)) gin_trgm_ops);

CREATE INDEX airports_iata_upper_idx ON airports (upper(iata));

CREATE INDEX airports_icao_upper_idx ON airports (upper(icao));
