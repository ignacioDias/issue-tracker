-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    avatar_url TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_email_not_empty
        CHECK (length(btrim(email)) > 0),

    CONSTRAINT users_email_lowercase
        CHECK (email = lower(email)),

    CONSTRAINT users_display_name_not_empty
        CHECK (length(btrim(display_name)) > 0)
);


CREATE TABLE IF NOT EXISTS oauth_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    provider TEXT NOT NULL,
    provider_subject TEXT NOT NULL,
    provider_email TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT oauth_provider_supported
        CHECK (provider IN ('google')),

    CONSTRAINT oauth_subject_not_empty
        CHECK (length(btrim(provider_subject)) > 0),

    CONSTRAINT oauth_provider_subject_unique
        UNIQUE (provider, provider_subject)
);

CREATE INDEX IF NOT EXISTS oauth_identities_user_idx
    ON oauth_identities (user_id);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    token_hash BYTEA NOT NULL UNIQUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    CONSTRAINT sessions_token_hash_length
        CHECK (octet_length(token_hash) = 32),

    CONSTRAINT sessions_expiration_valid
        CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS sessions_user_idx
    ON sessions (user_id);

CREATE INDEX IF NOT EXISTS sessions_active_expiration_idx
    ON sessions (expires_at)
    WHERE revoked_at IS NULL;


CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    owner_user_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT teams_name_not_empty
        CHECK (length(btrim(name)) > 0)
);

CREATE TABLE IF NOT EXISTS team_memberships (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- OWNER is represented by teams.owner_user_id.
    role TEXT NOT NULL DEFAULT 'MEMBER',

    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (team_id, user_id),

    CONSTRAINT team_memberships_role_valid
        CHECK (role IN ('ADMIN', 'MEMBER'))
);

CREATE INDEX IF NOT EXISTS team_memberships_user_idx
    ON team_memberships (user_id);

CREATE INDEX IF NOT EXISTS teams_created_by_idx
    ON teams (created_by);

-- The team's owner must also be one of its members.
-- Deferred because team and membership are inserted in one transaction.
ALTER TABLE teams
    ADD CONSTRAINT teams_owner_membership_fk
    FOREIGN KEY (id, owner_user_id)
    REFERENCES team_memberships (team_id, user_id)
    DEFERRABLE INITIALLY DEFERRED;


CREATE TABLE IF NOT EXISTS invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    token_hash BYTEA NOT NULL UNIQUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,

    used_at TIMESTAMPTZ,
    accepted_by UUID REFERENCES users(id) ON DELETE RESTRICT,
    revoked_at TIMESTAMPTZ,

    CONSTRAINT invitations_token_hash_length
        CHECK (octet_length(token_hash) = 32),

    CONSTRAINT invitations_expiration_valid
        CHECK (expires_at > created_at),

    CONSTRAINT invitations_used_consistent
        CHECK (
            (used_at IS NULL AND accepted_by IS NULL)
            OR
            (used_at IS NOT NULL AND accepted_by IS NOT NULL)
        ),

    CONSTRAINT invitations_not_used_and_revoked
        CHECK (NOT (used_at IS NOT NULL AND revoked_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS invitations_team_idx
    ON invitations (team_id, created_at DESC);

CREATE INDEX IF NOT EXISTS invitations_pending_idx
    ON invitations (expires_at)
    WHERE used_at IS NULL AND revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS invitations_created_by_idx
    ON invitations (created_by);

CREATE INDEX IF NOT EXISTS invitations_accepted_by_idx
    ON invitations (accepted_by);


CREATE TABLE IF NOT EXISTS issues (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,

    title TEXT NOT NULL,
    description TEXT,

    status TEXT NOT NULL DEFAULT 'TODO',
    priority TEXT NOT NULL DEFAULT 'MEDIUM',

    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    assignee_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,

    CONSTRAINT issues_title_valid
        CHECK (length(btrim(title)) BETWEEN 1 AND 200),

    CONSTRAINT issues_status_valid
        CHECK (status IN ('TODO', 'IN_PROGRESS', 'DONE')),

    CONSTRAINT issues_priority_valid
        CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'URGENT')),

    -- If an assignee exists, they must belong to this issue's team.
    CONSTRAINT issues_assignee_membership_fk
        FOREIGN KEY (team_id, assignee_id)
        REFERENCES team_memberships (team_id, user_id)
        ON DELETE SET NULL(assignee_id)
);

-- Main team issue listing, excluding archived issues.
CREATE INDEX IF NOT EXISTS issues_team_created_idx
    ON issues (team_id, created_at DESC, id DESC)
    WHERE archived_at IS NULL;

-- Useful for listing a user's assigned issues.
CREATE INDEX IF NOT EXISTS issues_assignee_idx
    ON issues (assignee_id, status)
    WHERE assignee_id IS NOT NULL AND archived_at IS NULL;


-- Keep updated_at current on every row update.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE OR REPLACE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER teams_set_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE OR REPLACE TRIGGER issues_set_updated_at
    BEFORE UPDATE ON issues
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();


-- +goose Down
DROP TABLE IF EXISTS issues;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS team_memberships;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS oauth_identities;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
