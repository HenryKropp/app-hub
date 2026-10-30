-- Every company has its own membership workflow, including the original one.
CREATE TABLE membership_invitations (
    id BIGSERIAL PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_by BIGINT NOT NULL REFERENCES people(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '7 days',
    revoked_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ
);
CREATE INDEX membership_invitations_email ON membership_invitations(email);

CREATE TABLE membership_requests (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by BIGINT REFERENCES people(id)
);
CREATE UNIQUE INDEX membership_requests_pending_email ON membership_requests(email) WHERE status='pending';
