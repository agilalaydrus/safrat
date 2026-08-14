-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('MANAGER', 'COORDINATOR', 'GROUP_LEADER');
CREATE TYPE plan AS ENUM ('STARTER', 'GROWTH', 'PRO');
CREATE TYPE season_type AS ENUM ('HAJJ', 'UMRAH');

CREATE TABLE operators (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  clerk_org_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  license_number TEXT,
  country CHAR(2) NOT NULL,
  phone TEXT,
  email TEXT NOT NULL,
  logo_url TEXT,
  plan plan NOT NULL DEFAULT 'STARTER',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  clerk_id TEXT NOT NULL UNIQUE,
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  role user_role NOT NULL,
  name TEXT NOT NULL,
  email TEXT NOT NULL,
  phone TEXT,
  avatar_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE seasons (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  type season_type NOT NULL,
  start_date TIMESTAMPTZ NOT NULL,
  end_date TIMESTAMPTZ NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT seasons_date_range_check CHECK (end_date >= start_date)
);

CREATE UNIQUE INDEX seasons_one_active_per_operator
  ON seasons(operator_id) WHERE is_active;
CREATE INDEX seasons_operator_id_idx ON seasons(operator_id);
CREATE INDEX users_operator_id_idx ON users(operator_id);

-- +goose Down
DROP TABLE IF EXISTS seasons;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS operators;
DROP TYPE IF EXISTS season_type;
DROP TYPE IF EXISTS plan;
DROP TYPE IF EXISTS user_role;
