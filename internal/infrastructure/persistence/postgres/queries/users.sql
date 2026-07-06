-- name: CreateUser :one
INSERT INTO "users" (
    uuid,
    first_name,
    last_name,
    email,
    login,
    created_at,
    updated_at,
    phone,
    password,
    is_active,
    birthday,
    extra_information,
    roles,
    agency_id
) VALUES (
    $1, $2, $3, $4, $5, NOW(), NOW(), $6, $7, $8, $9, $10, $11, $12
)
RETURNING id;

-- name: GetUserByEmail :one
SELECT id, uuid, first_name, last_name, email, login, created_at, updated_at,
       phone, password, is_active, birthday, extra_information, roles, agency_id
FROM "users"
WHERE email = $1;

-- name: GetUserByUuid :one
SELECT id, uuid, first_name, last_name, email, login, created_at, updated_at,
       phone, password, is_active, birthday, extra_information, roles, agency_id
FROM "users"
WHERE uuid = $1;

-- name: GetUserByID :one
SELECT id, uuid, first_name, last_name, email, login, created_at, updated_at,
       phone, password, is_active, birthday, extra_information, roles, agency_id
FROM "users"
WHERE id = $1;
