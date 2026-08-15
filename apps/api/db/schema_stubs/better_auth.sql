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
