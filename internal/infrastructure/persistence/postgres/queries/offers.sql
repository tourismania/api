-- name: CreateOffer :one
INSERT INTO offers (
    uuid, title, description, agency_id, created_by, status, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id;

-- name: GetOfferByUUID :one
SELECT id, uuid, title, description, agency_id, created_by, status, created_at, updated_at, deleted_at
FROM offers
WHERE uuid = $1 AND deleted_at IS NULL;

-- name: UpdateOffer :exec
UPDATE offers
SET title = $2, description = $3, status = $4, updated_at = $5
WHERE uuid = $1 AND deleted_at IS NULL;

-- name: SoftDeleteOffer :exec
UPDATE offers
SET deleted_at = NOW()
WHERE uuid = $1 AND deleted_at IS NULL;

-- name: ListOffers :many
-- Params: $1=agency_id (nullable), $2=status (nullable), $3=created_by (nullable), $4=limit, $5=offset.
-- Filters are optional: a NULL parameter means "no restriction on this column".
SELECT id, uuid, title, description, agency_id, created_by, status, created_at, updated_at, deleted_at,
       COUNT(*) OVER() AS total_count
FROM offers
WHERE deleted_at IS NULL
  AND ($1::int     IS NULL OR agency_id  = $1)
  AND ($2::varchar IS NULL OR status     = $2)
  AND ($3::int     IS NULL OR created_by = $3)
ORDER BY created_at DESC, id DESC
LIMIT $4 OFFSET $5;
