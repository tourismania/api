-- name: CreateOfferFlight :one
INSERT INTO offer_flights (offer_id, price_minor, currency, manager_comment)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListOfferFlightsByOfferID :many
SELECT id, offer_id, price_minor, currency, manager_comment
FROM offer_flights
WHERE offer_id = $1
ORDER BY id ASC;

-- name: UpdateOfferFlightDetails :exec
UPDATE offer_flights
SET price_minor = $3, currency = $4, manager_comment = $5
WHERE id = $1 AND offer_id = $2;

-- name: DeleteOfferFlightsByOfferID :exec
DELETE FROM offer_flights
WHERE offer_id = $1;
