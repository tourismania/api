-- name: CreateOfferFlightSegment :exec
INSERT INTO offer_flight_segments (
    flight_id, sequence, departure_airport_icao, arrival_airport_icao, departure_at, arrival_at, cabin_class
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);

-- name: ListOfferFlightSegmentsByFlightIDs :many
-- Params: $1=flight_ids (array). Ordered by flight then sequence so
-- callers can group consecutive rows by flight_id without a map lookup.
SELECT id, flight_id, sequence, departure_airport_icao, arrival_airport_icao, departure_at, arrival_at, cabin_class
FROM offer_flight_segments
WHERE flight_id = ANY($1::int[])
ORDER BY flight_id ASC, sequence ASC;

-- name: UpdateOfferFlightSegmentCabinClass :exec
-- Params: $1=flight_id, $2=offer_id, $3=sequence, $4=cabin_class. Меняет
-- только класс сегмента; аэропорты, время и id сегмента не трогаются.
-- Перелёт ищется в пределах offer_id, как в UpdateOfferFlightDetails:
-- сегменты перелёта чужого оффера не меняются.
UPDATE offer_flight_segments AS s
SET cabin_class = $4
FROM offer_flights AS f
WHERE s.flight_id = f.id AND f.id = $1 AND f.offer_id = $2 AND s.sequence = $3;
