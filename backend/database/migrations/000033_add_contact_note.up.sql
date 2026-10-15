-- vCard NOTE used to be unmapped and therefore stored in the hidden vcard_extra blob.
-- It is now a first-class column. Move existing values (several NOTEs are joined by a
-- blank line, as on import) and strip them from vcard_extra so they are not emitted twice.
ALTER TABLE contacts ADD COLUMN note TEXT DEFAULT '';

UPDATE contacts
SET note = (
  SELECT group_concat(json_extract(value, '$.Value'), char(10) || char(10))
  FROM json_each(vcard_extra, '$.properties.NOTE')
  WHERE json_extract(value, '$.Value') IS NOT NULL AND json_extract(value, '$.Value') != ''
)
WHERE vcard_extra IS NOT NULL AND vcard_extra != '' AND json_valid(vcard_extra)
  AND json_type(vcard_extra, '$.properties.NOTE') = 'array';

UPDATE contacts SET note = '' WHERE note IS NULL;

UPDATE contacts
SET vcard_extra = json_remove(vcard_extra, '$.properties.NOTE')
WHERE vcard_extra IS NOT NULL AND vcard_extra != '' AND json_valid(vcard_extra)
  AND json_type(vcard_extra, '$.properties.NOTE') IS NOT NULL;
