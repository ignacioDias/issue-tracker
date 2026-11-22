-- name: CreateOAuthIdentity :one
INSERT INTO oauth_identities (user_id, provider, provider_subject, provider_email)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOAuthIdentityByID :one
SELECT *
FROM oauth_identities
WHERE id = $1;

-- name: GetOAuthIdentityByProviderSubject :one
SELECT *
FROM oauth_identities
WHERE provider = $1
  AND provider_subject = $2;

-- name: ListOAuthIdentitiesByUser :many
SELECT *
FROM oauth_identities
WHERE user_id = $1
ORDER BY created_at;

-- name: UpdateOAuthIdentityEmail :one
UPDATE oauth_identities
SET provider_email = $2
WHERE id = $1
RETURNING *;

-- name: DeleteOAuthIdentity :execrows
DELETE FROM oauth_identities
WHERE id = $1;
