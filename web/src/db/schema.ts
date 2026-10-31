export const CURRENT_SCHEMA_VERSION = 4;

export const CREATE_SCHEMA_SQL = `
  CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
  );
  CREATE TABLE IF NOT EXISTS entries (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '[]',
    meta TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT NULL,
    local_dirty_at TEXT NULL,
    base_updated_at TEXT NULL
  );
  CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts3(
    title,
    url,
    note,
    tags
  );
  CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(docid, title, url, note, tags)
    VALUES (
      new.rowid,
      new.title,
      new.url,
      new.note,
      REPLACE(REPLACE(REPLACE(REPLACE(new.tags, '[', ' '), ']', ' '), '"', ' '), ',', ' ')
    );
  END;
  CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
    DELETE FROM entries_fts WHERE docid = old.rowid;
  END;
  CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
    DELETE FROM entries_fts WHERE docid = old.rowid;
    INSERT INTO entries_fts(docid, title, url, note, tags)
    VALUES (
      new.rowid,
      new.title,
      new.url,
      new.note,
      REPLACE(REPLACE(REPLACE(REPLACE(new.tags, '[', ' '), ']', ' '), '"', ' '), ',', ' ')
    );
  END;
  CREATE TABLE IF NOT EXISTS sync_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    cursor TEXT NOT NULL DEFAULT ''
  );
  INSERT OR IGNORE INTO sync_state (id, cursor) VALUES (1, '');
  INSERT OR REPLACE INTO schema_version (version) VALUES (${CURRENT_SCHEMA_VERSION});
`;

export function opfsDbFilename(username: string): string {
  return `linkomat.${username}.db`;
}

export function opfsSidecarPrefix(username: string): string {
  return `corrupt-${username}-`;
}