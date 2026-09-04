-- sqlc-only stub. Better Auth owns and migrates these tables itself
-- (`npx better-auth migrate`, see README.md) — goose never touches this file,
-- it exists purely so sqlc's query analyzer can resolve JOINs against the
-- real "user" table it doesn't otherwise know about. Keep in sync with
-- Better Auth's actual columns only as far as queries in db/query/ need them.
CREATE TABLE "user" (
  id text PRIMARY KEY,
  name text NOT NULL,
  email text NOT NULL,
  "emailVerified" boolean NOT NULL,
  image text,
  "createdAt" timestamptz NOT NULL,
  "updatedAt" timestamptz NOT NULL
);

CREATE TABLE "member" (
  id text PRIMARY KEY,
  "organizationId" text NOT NULL,
  "userId" text NOT NULL,
  role text NOT NULL,
  "createdAt" timestamptz NOT NULL
);

CREATE TABLE "session" (
  id text PRIMARY KEY,
  "expiresAt" timestamptz NOT NULL,
  token text NOT NULL,
  "createdAt" timestamptz NOT NULL,
  "updatedAt" timestamptz NOT NULL,
  "ipAddress" text,
  "userAgent" text,
  "userId" text NOT NULL,
  "activeOrganizationId" text
);
