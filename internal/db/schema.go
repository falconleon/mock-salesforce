// Package db provides SQLite storage for generated mock data.
package db

// Schema is the SQLite DDL for the mock data database.
const Schema = `
CREATE TABLE IF NOT EXISTS accounts (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    industry        TEXT NOT NULL,
    type            TEXT NOT NULL,
    website         TEXT,
    phone           TEXT,
    billing_city    TEXT,
    billing_state   TEXT,
    annual_revenue  REAL,
    num_employees   INTEGER,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS contacts (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL,
    phone           TEXT,
    title           TEXT,
    department      TEXT,
    is_primary      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id              TEXT PRIMARY KEY,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL,
    username        TEXT NOT NULL,
    title           TEXT,
    department      TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1,
    manager_id      TEXT REFERENCES users(id),
    user_role       TEXT,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cases (
    id              TEXT PRIMARY KEY,
    case_number     TEXT NOT NULL UNIQUE,
    subject         TEXT NOT NULL,
    description     TEXT NOT NULL,
    status          TEXT NOT NULL,
    priority        TEXT NOT NULL,
    product         TEXT,
    case_type       TEXT,
    origin          TEXT,
    reason          TEXT,
    owner_id        TEXT NOT NULL REFERENCES users(id),
    contact_id      TEXT NOT NULL REFERENCES contacts(id),
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    created_at      TEXT NOT NULL,
    closed_at       TEXT,
    is_closed       INTEGER NOT NULL DEFAULT 0,
    is_escalated    INTEGER NOT NULL DEFAULT 0,
    jira_issue_key  TEXT
);

CREATE TABLE IF NOT EXISTS email_messages (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL REFERENCES cases(id),
    subject         TEXT NOT NULL,
    text_body       TEXT NOT NULL,
    html_body       TEXT,
    from_address    TEXT NOT NULL,
    from_name       TEXT NOT NULL,
    to_address      TEXT NOT NULL,
    cc_address      TEXT,
    bcc_address     TEXT,
    message_date    TEXT NOT NULL,
    status          TEXT NOT NULL,
    incoming        INTEGER NOT NULL,
    has_attachment   INTEGER NOT NULL DEFAULT 0,
    headers         TEXT,
    sequence_num    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS case_comments (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL REFERENCES cases(id),
    comment_body    TEXT NOT NULL,
    created_by_id   TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL,
    is_published    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS feed_items (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL REFERENCES cases(id),
    body            TEXT NOT NULL,
    type            TEXT NOT NULL,
    created_by_id   TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jira_issues (
    id              TEXT PRIMARY KEY,
    key             TEXT NOT NULL UNIQUE,
    project_key     TEXT NOT NULL,
    summary         TEXT NOT NULL,
    description_adf TEXT NOT NULL,
    issue_type      TEXT NOT NULL,
    status          TEXT NOT NULL,
    priority        TEXT NOT NULL,
    assignee_id     TEXT,
    reporter_id     TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    labels          TEXT,
    sf_case_id      TEXT REFERENCES cases(id)
);

CREATE TABLE IF NOT EXISTS jira_comments (
    id              TEXT PRIMARY KEY,
    issue_id        TEXT NOT NULL REFERENCES jira_issues(id),
    author_id       TEXT NOT NULL,
    body_adf        TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jira_users (
    account_id      TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL,
    email           TEXT,
    account_type    TEXT NOT NULL DEFAULT 'atlassian',
    active          INTEGER NOT NULL DEFAULT 1,
    sf_user_id      TEXT REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS profile_images (
    id              TEXT PRIMARY KEY,
    persona_type    TEXT NOT NULL,
    persona_id      TEXT NOT NULL,
    image_path      TEXT NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    age             INTEGER,
    gender          TEXT,
    ethnicity       TEXT,
    hair_color      TEXT,
    hair_style      TEXT,
    eye_color       TEXT,
    glasses         INTEGER NOT NULL DEFAULT 0,
    facial_hair     TEXT,
    generated_at    TEXT,
    prompt          TEXT,
    UNIQUE(persona_type, persona_id)
);
`
