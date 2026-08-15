CREATE TABLE offer_flights (
    id       SERIAL PRIMARY KEY,
    offer_id INT    NOT NULL REFERENCES offers(id) ON DELETE CASCADE
);

CREATE INDEX offer_flights_offer_id_idx ON offer_flights (offer_id);
