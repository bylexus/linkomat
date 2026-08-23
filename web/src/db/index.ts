import { ref, type Ref } from 'vue';
import { LocalDb, type DbState } from './sqlite';

const dbState: Ref<DbState> = ref({ kind: 'pending' });
let instance: LocalDb | null = null;
let instanceUsername: string | null = null;

export function useDb(
  username: string | null
): { state: Ref<DbState>; db: LocalDb | null } {
  if (!username) {
    return { state: dbState, db: null };
  }
  if (instance && instanceUsername === username) {
    return { state: dbState, db: instance };
  }
  if (instance) {
    instance.close();
    instance = null;
  }
  instanceUsername = username;
  dbState.value = { kind: 'pending' };
  instance = new LocalDb(username);
  instance.open().then((s) => {
    dbState.value = s;
    const expose = import.meta.env.DEV || import.meta.env.VITE_EXPOSE_DEV;
    if (expose && s.kind === 'ready') {
      exposeDev();
    }
  });
  return { state: dbState, db: instance };
}

export function closeDb(): void {
  if (instance) {
    instance.close();
    instance = null;
    instanceUsername = null;
    dbState.value = { kind: 'pending' };
  }
}

function exposeDev(): void {
  if (!instance) return;
  const db = instance;
  (window as unknown as { __linkomatDb: unknown }).__linkomatDb = {
    raw: db.getRawDb(),
    listEntries: (
      opts?: { cursor?: { updated_at: string; id: string } | null }
    ) => db.listEntries(opts),
    searchEntries: (
      q: string,
      opts?: { cursor?: { updated_at: string; id: string } | null }
    ) => db.searchEntries(q, opts),
    getEntry: (id: string) => db.getEntry(id),
    upsertEntry: (input: {
      id: string;
      title: string;
      url: string;
      note: string;
      tags: string[];
    }) => db.upsertEntry(input),
    softDeleteEntry: (id: string) => db.softDeleteEntry(id),
    persist: () => db.persist(),
  };
}