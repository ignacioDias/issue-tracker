-- name: CreateTeamMembership :one
INSERT INTO team_memberships (team_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTeamMembership :one
SELECT *
FROM team_memberships
WHERE team_id = $1
  AND user_id = $2;

-- name: ListTeamMembers :many
SELECT *
FROM team_memberships
WHERE team_id = $1
ORDER BY joined_at, user_id;

-- name: UpdateTeamMemberRole :one
UPDATE team_memberships
SET role = $3
WHERE team_id = $1
  AND user_id = $2
RETURNING *;

-- name: DeleteTeamMembership :execrows
DELETE FROM team_memberships
WHERE team_id = $1
  AND user_id = $2;
