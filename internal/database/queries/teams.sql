-- Creating a team must also insert the owner's team_memberships row in the
-- same transaction, because teams_owner_membership_fk is deferred.

-- name: CreateTeam :one
INSERT INTO teams (name, created_by, owner_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTeamByID :one
SELECT *
FROM teams
WHERE id = $1;

-- name: ListTeamsByUser :many
SELECT t.*
FROM teams t
JOIN team_memberships m ON m.team_id = t.id
WHERE m.user_id = $1
ORDER BY t.created_at DESC, t.id DESC;

-- name: UpdateTeamName :one
UPDATE teams
SET name = $2
WHERE id = $1
RETURNING *;

-- name: DeleteTeam :execrows
DELETE FROM teams
WHERE id = $1;
