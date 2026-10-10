-- Full-text search (SQLite FTS5). Each index is a plain FTS5 table whose rowid is the id of
-- the row it indexes; triggers keep it in sync. unicode61 with remove_diacritics 2 folds
-- case and accents ("Zoë" matches "zoe"). Rows are indexed whatever their deleted_at: the
-- queries filter on the real table. user_id is never in the index, every query joins back.

CREATE VIRTUAL TABLE IF NOT EXISTS contacts_fts USING fts5(
    firstname, lastname, nickname, email, phone, emails, phones,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS contacts_fts_ai AFTER INSERT ON contacts BEGIN
    INSERT INTO contacts_fts(rowid, firstname, lastname, nickname, email, phone, emails, phones)
    VALUES (
        new.id, new.firstname, new.lastname, new.nickname, new.email, new.phone,
        COALESCE((SELECT group_concat(json_extract(j.value, '$.value'), ' ') FROM json_each(CASE WHEN json_valid(new.emails) THEN new.emails ELSE '[]' END) AS j), ''),
        COALESCE((SELECT group_concat(json_extract(j.value, '$.value'), ' ') FROM json_each(CASE WHEN json_valid(new.phones) THEN new.phones ELSE '[]' END) AS j), '')
    );
END;

CREATE TRIGGER IF NOT EXISTS contacts_fts_au AFTER UPDATE OF firstname, lastname, nickname, email, phone, emails, phones ON contacts BEGIN
    DELETE FROM contacts_fts WHERE rowid = old.id;
    INSERT INTO contacts_fts(rowid, firstname, lastname, nickname, email, phone, emails, phones)
    VALUES (
        new.id, new.firstname, new.lastname, new.nickname, new.email, new.phone,
        COALESCE((SELECT group_concat(json_extract(j.value, '$.value'), ' ') FROM json_each(CASE WHEN json_valid(new.emails) THEN new.emails ELSE '[]' END) AS j), ''),
        COALESCE((SELECT group_concat(json_extract(j.value, '$.value'), ' ') FROM json_each(CASE WHEN json_valid(new.phones) THEN new.phones ELSE '[]' END) AS j), '')
    );
END;

CREATE TRIGGER IF NOT EXISTS contacts_fts_ad AFTER DELETE ON contacts BEGIN
    DELETE FROM contacts_fts WHERE rowid = old.id;
END;

INSERT INTO contacts_fts(rowid, firstname, lastname, nickname, email, phone, emails, phones)
SELECT c.id, c.firstname, c.lastname, c.nickname, c.email, c.phone,
    COALESCE((SELECT group_concat(json_extract(j.value, '$.value'), ' ') FROM json_each(CASE WHEN json_valid(c.emails) THEN c.emails ELSE '[]' END) AS j), ''),
    COALESCE((SELECT group_concat(json_extract(j.value, '$.value'), ' ') FROM json_each(CASE WHEN json_valid(c.phones) THEN c.phones ELSE '[]' END) AS j), '')
FROM contacts c;

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
    content,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS notes_fts_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, content) VALUES (new.id, new.content);
END;
CREATE TRIGGER IF NOT EXISTS notes_fts_au AFTER UPDATE OF content ON notes BEGIN
    DELETE FROM notes_fts WHERE rowid = old.id;
    INSERT INTO notes_fts(rowid, content) VALUES (new.id, new.content);
END;
CREATE TRIGGER IF NOT EXISTS notes_fts_ad AFTER DELETE ON notes BEGIN
    DELETE FROM notes_fts WHERE rowid = old.id;
END;
INSERT INTO notes_fts(rowid, content) SELECT id, content FROM notes;

CREATE VIRTUAL TABLE IF NOT EXISTS activities_fts USING fts5(
    title, description, location,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS activities_fts_ai AFTER INSERT ON activities BEGIN
    INSERT INTO activities_fts(rowid, title, description, location) VALUES (new.id, new.title, new.description, new.location);
END;
CREATE TRIGGER IF NOT EXISTS activities_fts_au AFTER UPDATE OF title, description, location ON activities BEGIN
    DELETE FROM activities_fts WHERE rowid = old.id;
    INSERT INTO activities_fts(rowid, title, description, location) VALUES (new.id, new.title, new.description, new.location);
END;
CREATE TRIGGER IF NOT EXISTS activities_fts_ad AFTER DELETE ON activities BEGIN
    DELETE FROM activities_fts WHERE rowid = old.id;
END;
INSERT INTO activities_fts(rowid, title, description, location) SELECT id, title, description, location FROM activities;
