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
SELECT m.team_id,
       m.user_id,
       m.role,
       m.joined_at,
       u.email,
       u.display_name,
       u.avatar_url
FROM team_memberships m
JOIN users u ON u.id = m.user_id
WHERE m.team_id = $1
  AND (sqlc.narg(cursor_joined_at)::timestamptz IS NULL
       OR (m.joined_at, m.user_id) > (sqlc.narg(cursor_joined_at)::timestamptz, sqlc.narg(cursor_user_id)::uuid))
ORDER BY m.joined_at, m.user_id
LIMIT sqlc.arg(page_limit);

-- name: UpdateTeamMemberRole :one
UPDATE team_memberships
SET role = $3
WHERE team_id = $1
  AND user_id = $2
RETURNING *;

-- The owner's membership row cannot be deleted: teams_owner_membership_fk
-- would fail at commit. Transfer ownership before removing the owner.
-- name: DeleteTeamMembership :execrows
DELETE FROM team_memberships m
USING teams t
WHERE m.team_id = $1
  AND m.user_id = $2
  AND t.id = m.team_id
  AND t.owner_user_id <> m.user_id;
