-- name: CreateOfferFlight :one
INSERT INTO offer_flights (offer_id)
VALUES ($1)
RETURNING id;

-- name: ListOfferFlightsByOfferID :many
SELECT id, offer_id
FROM offer_flights
WHERE offer_id = $1
ORDER BY id ASC;

-- name: DeleteOfferFlightsByOfferID :exec
DELETE FROM offer_flights
WHERE offer_id = $1;
