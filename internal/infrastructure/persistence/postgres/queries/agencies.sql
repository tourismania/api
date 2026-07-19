-- name: CreateAgency :one
INSERT INTO agencies (
    uuid, name, status, created_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING id;

-- name: GetAgencyByID :one
SELECT id, uuid, name, status, created_at, deleted_at
FROM agencies
WHERE id = $1 AND deleted_at IS NULL;

-- name: SetAgencyStatus :exec
UPDATE agencies
SET status = $2
WHERE id = $1;

-- name: AgencyExists :one
SELECT EXISTS (
    SELECT 1 FROM agencies WHERE id = $1 AND deleted_at IS NULL
);
