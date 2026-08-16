import { ref, type Ref } from 'vue';
import { LocalDb, type DbState } from './sqlite';

const dbState: Ref<DbState> = ref({ kind: 'pending' });
let instance: LocalDb | null = null;

export function useDb(): { state: Ref<DbState>; db: LocalDb | null } {
  if (!instance) {
    instance = new LocalDb();
    instance.open().then((s) => {
      dbState.value = s;
      const expose = import.meta.env.DEV || import.meta.env.VITE_EXPOSE_DEV;
      if (expose && s.kind === 'ready') {
        exposeDev();
      }
    });
  }
  return { state: dbState, db: instance };
}

function exposeDev(): void {
  if (!instance) return;
  const db = instance;
  (window as unknown as { __linkomatDb: unknown }).__linkomatDb = {
    raw: db.getRawDb(),
    listEntries: () => db.listEntries(),
    searchEntries: (q: string) => db.searchEntries(q),
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