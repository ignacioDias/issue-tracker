-- name: CreateInvitation :one
INSERT INTO invitations (team_id, created_by, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetInvitationByID :one
SELECT *
FROM invitations
WHERE id = $1;

-- name: GetInvitationByTokenHash :one
SELECT *
FROM invitations
WHERE token_hash = $1;

-- name: ListInvitationsByTeam :many
SELECT *
FROM invitations
WHERE team_id = $1
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- Accepts the invitation and adds the user to the team in one statement.
-- Returns no row if the invitation is used, revoked, or expired.
-- name: AcceptInvitation :one
WITH accepted AS (
    UPDATE invitations
    SET used_at = now(),
        accepted_by = $2
    WHERE id = $1
      AND used_at IS NULL
      AND revoked_at IS NULL
      AND expires_at > now()
    RETURNING *
),
membership AS (
    INSERT INTO team_memberships (team_id, user_id, role)
    SELECT team_id, $2, 'MEMBER'
    FROM accepted
    ON CONFLICT (team_id, user_id) DO NOTHING
    RETURNING team_id
)
SELECT *
FROM accepted;

-- name: RevokeInvitation :execrows
UPDATE invitations
SET revoked_at = now()
WHERE id = $1
  AND used_at IS NULL
  AND revoked_at IS NULL;

-- name: DeleteInvitation :execrows
DELETE FROM invitations
WHERE id = $1;
