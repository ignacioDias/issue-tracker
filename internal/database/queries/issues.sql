-- name: CreateIssue :one
INSERT INTO issues (team_id, title, description, status, priority, created_by, assignee_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetIssueByID :one
SELECT *
FROM issues
WHERE id = $1;

-- name: ListActiveIssuesByTeam :many
SELECT *
FROM issues
WHERE team_id = $1
  AND archived_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: UpdateIssue :one
UPDATE issues
SET title = $2,
    description = $3,
    status = $4,
    priority = $5,
    assignee_id = $6
WHERE id = $1
RETURNING *;

-- name: ArchiveIssue :execrows
UPDATE issues
SET archived_at = now()
WHERE id = $1
  AND archived_at IS NULL;

-- name: DeleteIssue :execrows
DELETE FROM issues
WHERE id = $1;
