-- Invited accounts exist before their owner has chosen a password. They are
-- created with an unusable password hash and may only sign in after consuming
-- their invitation token, so the pending state has to be recorded explicitly:
-- an unusable hash is indistinguishable from a bcrypt hash at the SQL level.
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_set_password BOOLEAN NOT NULL DEFAULT FALSE;

-- Records who invited an account, for display in the users list. Kept as a
-- nullable reference so removing the inviting account does not remove the
-- accounts they created.
ALTER TABLE users ADD COLUMN IF NOT EXISTS invited_by TEXT REFERENCES users(id) ON DELETE SET NULL;

-- The role default was 'owner' because the first account was the only account.
-- A new row now has to name its role, so that adding a user without one fails
-- loudly rather than silently creating a second owner.
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
