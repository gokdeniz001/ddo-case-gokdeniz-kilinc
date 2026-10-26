CREATE TABLE "fy-refresh-sessions" (
    token_hash text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES "fy-ox-users"(id) ON DELETE CASCADE,
    site_id uuid NOT NULL REFERENCES "fy-sites"(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_session_user ON "fy-refresh-sessions"(user_id);
