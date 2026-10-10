-- A purged contact leaves its CardDAV link behind as a tombstone: pull-only syncs
-- must never import the remote object again, two-way syncs delete it remotely.
ALTER TABLE carddav_contact_links ADD COLUMN purged_at DATETIME;

-- Links whose contact row is already gone were left by earlier purges.
UPDATE carddav_contact_links SET purged_at = CURRENT_TIMESTAMP
WHERE contact_id NOT IN (SELECT id FROM contacts);
