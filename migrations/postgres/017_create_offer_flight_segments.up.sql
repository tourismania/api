CREATE TABLE offer_flight_segments (
    id                     SERIAL    PRIMARY KEY,
    flight_id              INT       NOT NULL REFERENCES offer_flights(id) ON DELETE CASCADE,
    sequence               SMALLINT  NOT NULL,
    departure_airport_icao CHAR(4)   NOT NULL REFERENCES airports(icao),
    arrival_airport_icao   CHAR(4)   NOT NULL REFERENCES airports(icao),
    departure_at           TIMESTAMP NOT NULL,
    arrival_at             TIMESTAMP NOT NULL,
    UNIQUE (flight_id, sequence)
);

CREATE INDEX offer_flight_segments_flight_id_idx ON offer_flight_segments (flight_id);
