-- name: CreateOfferFlight :one
INSERT INTO offer_flights (offer_id, price_minor, currency)
VALUES ($1, $2, $3)
RETURNING id;

-- name: ListOfferFlightsByOfferID :many
SELECT id, offer_id, price_minor, currency
FROM offer_flights
WHERE offer_id = $1
ORDER BY id ASC;

-- name: UpdateOfferFlightPrice :exec
UPDATE offer_flights
SET price_minor = $3, currency = $4
WHERE id = $1 AND offer_id = $2;

-- name: DeleteOfferFlightsByOfferID :exec
DELETE FROM offer_flights
WHERE offer_id = $1;
