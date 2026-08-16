import initSqlJs, { type Database, type SqlJsStatic } from 'sql.js';
import {
  opfsAvailable,
  readOpfsFile,
  writeOpfsFile,
  renameOpfsFile,
  sidecarName,
} from './opfs';
import {
  CURRENT_SCHEMA_VERSION,
  CREATE_SCHEMA_SQL,
  OPFS_DB_FILENAME,
  OPFS_SIDECAR_PREFIX,
} from './schema';

let sqlPromise: Promise<SqlJsStatic> | null = null;

function loadSqlJs(): Promise<SqlJsStatic> {
  if (sqlPromise) return sqlPromise;
  sqlPromise = initSqlJs({
    locateFile: (file: string) => `/sqlite/${file}?v=${SQLJS_ASSET_VERSION}`,
  });
  return sqlPromise;
}

const SQLJS_ASSET_VERSION = 2;

export type DbState =
  | { kind: 'pending' }
  | { kind: 'opfs-unavailable' }
  | {
      kind: 'ready';
      fresh: boolean;
      integrityOk: boolean;
      schemaVersion: number;
      sidecarName?: string;
      errorMessage?: string;
    }
  | { kind: 'error'; message: string };

export type Entry = {
  id: string;
  title: string;
  url: string;
  note: string;
  tags: string[];
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
  local_dirty_at: string | null;
  base_updated_at: string | null;
};

type EntryRow = Omit<Entry, 'tags'> & { tags: string };

export class LocalDb {
  private db: Database | null = null;
  private sql: SqlJsStatic | null = null;
  state: DbState = { kind: 'pending' };

  async open(): Promise<DbState> {
    if (!opfsAvailable()) {
      this.state = { kind: 'opfs-unavailable' };
      return this.state;
    }

    try {
      this.sql = await loadSqlJs();
    } catch (err) {
      this.state = { kind: 'error', message: errorMessage(err, 'sql.js init failed') };
      return this.state;
    }

    let stored: Uint8Array | null = null;
    try {
      stored = await readOpfsFile(OPFS_DB_FILENAME);
    } catch (err) {
      this.state = { kind: 'error', message: errorMessage(err, 'OPFS read failed') };
      return this.state;
    }

    let recoveredFromCorruption = false;
    let recoveredFromSchema = false;
    let sidecar: string | undefined;

    if (stored && stored.length > 0) {
      try {
        this.db = new this.sql.Database(stored);
      } catch (err) {
        this.state = { kind: 'error', message: errorMessage(err, 'sqlite open failed') };
        return this.state;
      }

      const integrity = this.checkIntegrity();
      if (!integrity.ok) {
        sidecar = sidecarName(OPFS_SIDECAR_PREFIX);
        await renameOpfsFile(OPFS_DB_FILENAME, sidecar).catch(() => undefined);
        this.db.close();
        this.db = new this.sql.Database();
        this.createSchema();
        recoveredFromCorruption = true;
        await this.persist().catch(() => undefined);
        this.state = {
          kind: 'ready',
          fresh: true,
          integrityOk: false,
          schemaVersion: CURRENT_SCHEMA_VERSION,
          sidecarName: sidecar,
          errorMessage: 'Local database was corrupt; rebuilt from scratch.',
        };
        return this.state;
      }

      const v = this.getSchemaVersion();
      if (v !== CURRENT_SCHEMA_VERSION) {
        sidecar = sidecarName(`${OPFS_SIDECAR_PREFIX}v${v}-`);
        await renameOpfsFile(OPFS_DB_FILENAME, sidecar).catch(() => undefined);
        this.db.close();
        this.db = new this.sql.Database();
        this.createSchema();
        recoveredFromSchema = true;
        await this.persist().catch(() => undefined);
        this.state = {
          kind: 'ready',
          fresh: true,
          integrityOk: true,
          schemaVersion: CURRENT_SCHEMA_VERSION,
          sidecarName: sidecar,
          errorMessage: `Local database schema v${v} not supported; rebuilt.`,
        };
        return this.state;
      }

      this.state = {
        kind: 'ready',
        fresh: false,
        integrityOk: true,
        schemaVersion: v,
      };
      return this.state;
    }

    this.db = new this.sql.Database();
    this.createSchema();
    await this.persist().catch(() => undefined);
    this.state = {
      kind: 'ready',
      fresh: true,
      integrityOk: true,
      schemaVersion: CURRENT_SCHEMA_VERSION,
    };
    return this.state;

    void recoveredFromCorruption;
    void recoveredFromSchema;
  }

  private checkIntegrity(): { ok: boolean; report?: string } {
    if (!this.db) return { ok: false, report: 'no database' };
    const result = this.db.exec('PRAGMA integrity_check');
    if (result.length === 0 || result[0].values.length === 0) {
      return { ok: false, report: 'empty result' };
    }
    const first = result[0].values[0][0];
    return { ok: first === 'ok', report: String(first) };
  }

  private getSchemaVersion(): number {
    if (!this.db) return 0;
    const result = this.db.exec('SELECT version FROM schema_version LIMIT 1');
    if (result.length === 0 || result[0].values.length === 0) return 0;
    const v = result[0].values[0][0];
    return typeof v === 'number' ? v : Number(v) || 0;
  }

  private createSchema(): void {
    if (!this.db) throw new Error('db not open');
    this.db.exec(CREATE_SCHEMA_SQL);
  }

  async persist(): Promise<void> {
    if (!this.db) throw new Error('db not open');
    const data = this.db.export();
    await writeOpfsFile(OPFS_DB_FILENAME, data);
  }

  getRawDb(): Database {
    if (!this.db) throw new Error('db not open');
    return this.db;
  }

  listEntries(): Entry[] {
    if (!this.db) throw new Error('db not open');
    const stmt = this.db.prepare(`
      SELECT id, title, url, note, tags, created_at, updated_at, deleted_at, local_dirty_at, base_updated_at
      FROM entries
      WHERE deleted_at IS NULL
      ORDER BY updated_at DESC, id ASC
    `);
    const out: Entry[] = [];
    while (stmt.step()) {
      out.push(rowToEntry(stmt.getAsObject() as EntryRow));
    }
    stmt.free();
    return out;
  }

  searchEntries(query: string): Entry[] {
    if (!this.db) throw new Error('db not open');
    const fts = toFtsQuery(query);
    if (!fts) return this.listEntries();
    const stmt = this.db.prepare(`
      SELECT e.id, e.title, e.url, e.note, e.tags, e.created_at, e.updated_at, e.deleted_at, e.local_dirty_at, e.base_updated_at
      FROM entries_fts f, entries e
      WHERE f.docid = e.rowid
        AND entries_fts MATCH ?
        AND e.deleted_at IS NULL
      ORDER BY e.updated_at DESC, e.id ASC
    `);
    stmt.bind([fts]);
    const out: Entry[] = [];
    while (stmt.step()) {
      out.push(rowToEntry(stmt.getAsObject() as EntryRow));
    }
    stmt.free();
    return out;
  }

  close(): void {
    this.db?.close();
    this.db = null;
  }
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof Error) return `${fallback}: ${err.message}`;
  return `${fallback}: ${String(err)}`;
}

function rowToEntry(row: EntryRow): Entry {
  let tags: string[];
  try {
    const parsed = JSON.parse(row.tags);
    tags = Array.isArray(parsed) ? parsed.filter((t): t is string => typeof t === 'string') : [];
  } catch {
    tags = [];
  }
  return { ...row, tags };
}

function toFtsQuery(input: string): string {
  const tokens = input
    .trim()
    .split(/\s+/)
    .map((t) => t.replace(/["']/g, ''))
    .filter(Boolean);
  if (tokens.length === 0) return '';
  return tokens.map((t) => {
    const upper = t.toUpperCase();
    if (upper === 'AND' || upper === 'OR' || upper === 'NOT' || upper === 'NEAR') return upper;
    return `${t}*`;
  }).join(' ');
}