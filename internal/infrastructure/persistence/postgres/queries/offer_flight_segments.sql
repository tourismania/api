-- name: CreateOfferFlightSegment :exec
INSERT INTO offer_flight_segments (
    flight_id, sequence, departure_airport_icao, arrival_airport_icao, departure_at, arrival_at
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: ListOfferFlightSegmentsByFlightIDs :many
-- Params: $1=flight_ids (array). Ordered by flight then sequence so
-- callers can group consecutive rows by flight_id without a map lookup.
SELECT id, flight_id, sequence, departure_airport_icao, arrival_airport_icao, departure_at, arrival_at
FROM offer_flight_segments
WHERE flight_id = ANY($1::int[])
ORDER BY flight_id ASC, sequence ASC;
