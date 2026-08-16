import { watch } from 'vue';
import { useAuth } from '../state/auth';
import { useDb } from '../db';
import { useReachability, startReachability } from './reachability';
import { setSyncState, syncStateRef, type SyncSkipReason } from '../state/sync';
import {
  listEntries,
  putEntry,
  deleteEntry,
  type ServerEntry,
} from '../api/entries';
import { DB_CHANGE_EVENT } from '../db/sqlite';

const PULL_PAGE_SIZE = 200;
const PULL_MAX_PAGES = 200;
const SYNC_INTERVAL_MS = 1 * 60_000;
const PUSH_DEBOUNCE_MS = 2_000;
const BACKOFF_BASE_MS = 1_000;
const BACKOFF_MAX_MS = 60_000;
const BACKOFF_JITTER = 0.2;
const PERSISTENT_FAILURE_MS = 5 * 60_000;

let syncInFlight = false;
let initialized = false;
let intervalTimer: number | null = null;
let pushTimer: number | null = null;
let backoffTimer: number | null = null;
let backoffAttempt = 0;
let firstFailureAt: number | null = null;

function nextBackoffDelay(): number {
  const base = Math.min(BACKOFF_MAX_MS, BACKOFF_BASE_MS * Math.pow(2, backoffAttempt));
  const jitter = 1 + (Math.random() * 2 - 1) * BACKOFF_JITTER;
  return Math.round(base * jitter);
}

function scheduleBackoff(): void {
  if (typeof window === 'undefined') return;
  if (firstFailureAt === null) {
    firstFailureAt = Date.now();
  }
  backoffAttempt++;
  if (backoffTimer !== null) clearTimeout(backoffTimer);
  const delay = nextBackoffDelay();
  backoffTimer = window.setTimeout(() => {
    backoffTimer = null;
    if (firstFailureAt !== null && Date.now() - firstFailureAt > PERSISTENT_FAILURE_MS) {
      setSyncState({ persistentFailure: true });
    }
    void syncOnce();
  }, delay);
}

function resetBackoff(): void {
  if (backoffTimer !== null) {
    clearTimeout(backoffTimer);
    backoffTimer = null;
  }
  backoffAttempt = 0;
  firstFailureAt = null;
  setSyncState({ persistentFailure: false });
}

function logError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

async function getActiveDb(): Promise<ReturnType<typeof useDb> | null> {
  const { username } = useAuth();
  if (!username.value) return null;
  return useDb(username.value);
}

async function refreshPendingCount(): Promise<number> {
  const db = await getActiveDb();
  if (!db || !db.db) return 0;
  if (db.state.value.kind !== 'ready') return 0;
  const count = db.db.countDirtyEntries();
  setSyncState({ pendingCount: count });
  return count;
}

async function pushDirtyEntries(): Promise<void> {
  const db = await getActiveDb();
  if (!db || !db.db) return;
  if (db.state.value.kind !== 'ready') return;
  const dirty = db.db.listDirtyEntries();
  for (const entry of dirty) {
    if (entry.deleted_at) {
      const res = await deleteEntry(entry.id);
      if (res.kind === 'ok' || (res.kind === 'http-error' && res.status === 404)) {
        await db.db.clearDirty(entry.id, entry.updated_at);
      } else {
        throw new Error(
          `push DELETE ${entry.id} failed: ${res.kind === 'http-error' ? res.status : res.kind}`
        );
      }
    } else {
      const res = await putEntry(entry.id, {
        title: entry.title,
        url: entry.url,
        note: entry.note,
        tags: entry.tags,
      });
      if (res.kind === 'ok') {
        await db.db.clearDirty(entry.id, res.body.updated_at);
      } else {
        throw new Error(
          `push PUT ${entry.id} failed: ${res.kind === 'http-error' ? res.status : res.kind}`
        );
      }
    }
  }
}

async function fullPull(): Promise<string> {
  const all: ServerEntry[] = [];
  let lastServerTime = '';
  for (let page = 0; page < PULL_MAX_PAGES; page++) {
    const res = await listEntries({
      limit: PULL_PAGE_SIZE,
      offset: page * PULL_PAGE_SIZE,
    });
    if (res.kind !== 'ok') {
      throw new Error(
        `full pull failed: ${res.kind === 'http-error' ? res.status : res.kind}`
      );
    }
    all.push(...res.body.entries);
    lastServerTime = res.body.server_time;
    if (res.body.entries.length < PULL_PAGE_SIZE) break;
  }
  const db = await getActiveDb();
  if (!db || !db.db) return lastServerTime;
  if (db.state.value.kind !== 'ready') return lastServerTime;
  await db.db.applyServerEntries(all);
  return lastServerTime;
}

async function deltaPull(cursor: string): Promise<string> {
  const res = await listEntries({ changed_since: cursor });
  if (res.kind !== 'ok') {
    throw new Error(
      `delta pull failed: ${res.kind === 'http-error' ? res.status : res.kind}`
    );
  }
  const db = await getActiveDb();
  if (!db || !db.db) return res.body.server_time;
  if (db.state.value.kind !== 'ready') return res.body.server_time;
  await db.db.applyServerEntries(res.body.entries);
  return res.body.server_time;
}

async function applyCursor(serverTime: string): Promise<void> {
  const db = await getActiveDb();
  if (!db || !db.db) return;
  if (db.state.value.kind !== 'ready') return;
  db.db.setCursor(serverTime);
  await db.db.persist();
}

async function waitForDbReady(timeoutMs = 30_000): Promise<ReturnType<typeof useDb> | null> {
  const db = await getActiveDb();
  if (!db || !db.db) return null;
  if (db.state.value.kind === 'ready') return db;
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      stop();
      resolve(null);
    }, timeoutMs);
    const stop = watch(
      () => db.state.value,
      (s) => {
        if (s.kind === 'ready') {
          clearTimeout(timer);
          stop();
          resolve(db);
        } else if (s.kind === 'error' || s.kind === 'opfs-unavailable') {
          clearTimeout(timer);
          stop();
          resolve(null);
        }
      }
    );
  });
}

function skip(reason: SyncSkipReason): void {
  setSyncState({ lastSkipReason: reason });
}

async function syncOnce(): Promise<void> {
  if (syncInFlight) return skip('in-flight');
  const { username } = useAuth();
  if (!username.value) return skip('no-username');
  const online = useReachability();
  if (!online.value) return skip('offline');
  const db = await waitForDbReady();
  if (!db || !db.db) return skip('db-not-ready');

  syncInFlight = true;
  setSyncState({ syncing: true, lastSkipReason: null });
  let ok = false;
  try {
    await pushDirtyEntries();
    const cursor = db.db.getCursor();
    const serverTime = cursor === null ? await fullPull() : await deltaPull(cursor);
    await applyCursor(serverTime);
    setSyncState({ lastError: null, lastSyncAt: new Date().toISOString() });
    resetBackoff();
    ok = true;
  } catch (err) {
    setSyncState({ lastError: logError(err) });
    scheduleBackoff();
  } finally {
    setSyncState({ syncing: false });
    syncInFlight = false;
    const pending = await refreshPendingCount();
    if (ok && pending > 0) {
      schedulePush();
    }
  }
}

function onVisibilityChange(): void {
  if (typeof document === 'undefined') return;
  if (document.visibilityState === 'visible') {
    void syncOnce();
  }
}

function onOnlineEvent(): void {
  void syncOnce();
}

function onDbChange(): void {
  void refreshPendingCount();
  if (syncInFlight) return;
  schedulePush();
}

function schedulePush(): void {
  if (typeof window === 'undefined') return;
  if (pushTimer !== null) clearTimeout(pushTimer);
  pushTimer = window.setTimeout(() => {
    pushTimer = null;
    void syncOnce();
  }, PUSH_DEBOUNCE_MS);
}

function cancelPush(): void {
  if (pushTimer !== null) {
    clearTimeout(pushTimer);
    pushTimer = null;
  }
}

function startInterval(): void {
  if (intervalTimer !== null) return;
  if (typeof window === 'undefined') return;
  intervalTimer = window.setInterval(() => {
    void syncOnce();
  }, SYNC_INTERVAL_MS);
}

function stopInterval(): void {
  if (intervalTimer !== null) {
    clearInterval(intervalTimer);
    intervalTimer = null;
  }
}

function attachListeners(): void {
  if (typeof window === 'undefined') return;
  window.addEventListener('online', onOnlineEvent);
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibilityChange);
  }
  window.addEventListener(DB_CHANGE_EVENT, onDbChange);
}

function detachListeners(): void {
  cancelPush();
  if (typeof window === 'undefined') return;
  window.removeEventListener('online', onOnlineEvent);
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', onVisibilityChange);
  }
  window.removeEventListener(DB_CHANGE_EVENT, onDbChange);
}

export function initSync(): void {
  if (initialized) return;
  initialized = true;
  startReachability();
  const online = useReachability();
  setSyncState({ online: online.value });
  watch(online, (v) => {
    setSyncState({ online: v });
    if (v) {
      void syncOnce();
    }
  });
  const { state: authState } = useAuth();
  watch(
    authState,
    (s) => {
      if (s === 'logged-in') {
        attachListeners();
        startInterval();
        const expose = import.meta.env.DEV || import.meta.env.VITE_EXPOSE_DEV;
        if (expose) {
          exposeDev();
        }
        setSyncState({ online: online.value });
        void syncOnce();
      } else {
        detachListeners();
        stopInterval();
        setSyncState({
          syncing: false,
          pendingCount: 0,
          lastError: null,
          lastSkipReason: null,
          lastSyncAt: null,
          persistentFailure: false,
        });
        resetBackoff();
      }
    },
    { immediate: true }
  );
}

export function syncNow(): Promise<void> {
  return syncOnce();
}

function exposeDev(): void {
  if (typeof window === 'undefined') return;
  (window as unknown as { __linkomatSync: unknown }).__linkomatSync = {
    syncNow,
    reachability: useReachability(),
    state: syncStateRef(),
  };
}
