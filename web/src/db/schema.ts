export const CURRENT_SCHEMA_VERSION = 1;

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
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT NULL,
    local_dirty_at TEXT NULL,
    base_updated_at TEXT NULL
  );
  INSERT OR REPLACE INTO schema_version (version) VALUES (${CURRENT_SCHEMA_VERSION});
`;

export const OPFS_DB_FILENAME = 'linkomat.db';

export const OPFS_SIDECAR_PREFIX = 'corrupt-';