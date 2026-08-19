-- +goose Up

-- Staff assignment per kloter. A staff member can be assigned to multiple
-- kloters, a kloter can have multiple staff with different roles.
CREATE TABLE kloter_staff (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id UUID        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  kloter_id   UUID        NOT NULL REFERENCES kloters(id)  ON DELETE CASCADE,
  staff_id    TEXT        NOT NULL,   -- Better Auth "user"(id), TEXT not UUID
  staff_name  TEXT        NOT NULL,   -- denormalized for display without join
  staff_email TEXT        NOT NULL,
  role        TEXT        NOT NULL DEFAULT 'COORDINATOR'
              CHECK (role IN ('COORDINATOR','MEDICAL','GUIDE','ADMIN_SUPPORT')),
  duties      TEXT        NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (kloter_id, staff_id)
);

CREATE INDEX kloter_staff_operator_idx ON kloter_staff(operator_id);
CREATE INDEX kloter_staff_kloter_idx   ON kloter_staff(kloter_id);
CREATE INDEX kloter_staff_user_idx     ON kloter_staff(staff_id);

-- +goose Down
DROP TABLE IF EXISTS kloter_staff;
