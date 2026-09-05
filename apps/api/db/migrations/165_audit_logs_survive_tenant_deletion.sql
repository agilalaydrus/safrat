-- +goose Up
-- D6 (TUGAS-PANEL-SAAS.md, §7.3 DESAIN): "yang dihapus adalah data pribadi;
-- audit_logs tetap, karena ia bukti dan retensinya 24 bulan (migrasi 126).
-- Menghapus jejak audit bersama tenantnya akan menghapus justru catatan
-- yang membuktikan penghapusan itu sah."
--
-- Every table in this schema that belongs to a tenant already cascades from
-- operators(id) ON DELETE CASCADE — that is what makes DELETE FROM operators
-- a complete, correct deletion of a tenant's data with nothing left orphaned
-- behind it. audit_logs is the one deliberate exception: it must outlive the
-- tenant it was written about, so its FK changes from CASCADE to SET NULL.
--
-- operator_id is already nullable (migration 108, for platform-level
-- actions that belong to no tenant) — this migration only changes what
-- happens to a tenant-scoped row once its tenant is gone: it now reads as
-- "this event was about a tenant that no longer exists," the same NULL
-- migration 108 already uses for "never had a tenant to begin with."
ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_operator_id_fkey;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_operator_id_fkey
  FOREIGN KEY (operator_id) REFERENCES operators(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_operator_id_fkey;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_operator_id_fkey
  FOREIGN KEY (operator_id) REFERENCES operators(id) ON DELETE CASCADE;
