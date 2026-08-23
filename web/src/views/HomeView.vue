<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { onBeforeRouteLeave, useRouter } from 'vue-router';
import { useDb } from '../db';
import { DB_CHANGE_EVENT } from '../db/sqlite';
import { uuidv7 } from '../utils/uuid';
import { normalizeUrl } from '../utils/url';
import { useAuth } from '../state/auth';
import { useSyncState } from '../state/sync';
import { usePreservedHomeState } from '../state/homeState';
import { syncNow } from '../sync/engine';
import type { Entry } from '../db/sqlite';

const { username, logout } = useAuth();
const { state, db } = useDb(username.value);
const router = useRouter();

const syncState = useSyncState();

const {
  query: search,
  numPages: preservedNumPages,
  scrollY: preservedScrollY,
  isPreserved,
} = usePreservedHomeState();

const debouncedSearch = ref('');
const searchInput = ref<HTMLInputElement | null>(null);
const isLoadingMore = ref(false);
const entries = ref<Entry[]>([]);
const cursor = ref<{ updated_at: string; id: string } | null>(null);
const hasMore = ref(false);

let debounceTimer: number | null = null;
watch(search, (q) => {
  if (debounceTimer !== null) clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    debouncedSearch.value = q;
  }, 200);
});

function reload(): void {
  if (!db || state.value.kind !== 'ready') {
    entries.value = [];
    cursor.value = null;
    hasMore.value = false;
    return;
  }
  const q = debouncedSearch.value.trim();
  const page = q ? db.searchEntries(q) : db.listEntries();
  entries.value = page.entries;
  hasMore.value = page.hasMore;
  cursor.value =
    page.hasMore && page.entries.length > 0
      ? {
          updated_at: page.entries[page.entries.length - 1].updated_at,
          id: page.entries[page.entries.length - 1].id,
        }
      : null;
}

async function loadMore(): Promise<void> {
  if (isLoadingMore.value || !hasMore.value) return;
  if (!db || state.value.kind !== 'ready') return;
  if (!cursor.value) return;
  isLoadingMore.value = true;
  try {
    const q = debouncedSearch.value.trim();
    const opts = { cursor: cursor.value };
    const page = q ? db.searchEntries(q, opts) : db.listEntries(opts);
    entries.value = [...entries.value, ...page.entries];
    hasMore.value = page.hasMore;
    cursor.value =
      page.hasMore && page.entries.length > 0
        ? {
            updated_at: page.entries[page.entries.length - 1].updated_at,
            id: page.entries[page.entries.length - 1].id,
          }
        : null;
  } finally {
    isLoadingMore.value = false;
  }
}

let skipNextReload = 0;
watch([debouncedSearch, state, username], () => {
  if (skipNextReload > 0) {
    skipNextReload--;
    return;
  }
  reload();
});

function onChange(): void {
  if (!db || state.value.kind !== 'ready') return;
  for (let i = 0; i < entries.value.length; i++) {
    const fresh = db.getEntry(entries.value[i].id);
    if (fresh) {
      entries.value[i] = fresh;
    }
  }
}

onBeforeRouteLeave(async (to) => {
  if (!to.path.startsWith('/entries/')) return;
  if (debounceTimer !== null) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
    debouncedSearch.value = search.value;
    await nextTick();
  }
  preservedNumPages.value = Math.max(1, Math.ceil(entries.value.length / 50));
  preservedScrollY.value = window.scrollY;
  isPreserved.value = true;
});

onMounted(async () => {
  if (isPreserved.value) {
    isPreserved.value = false;
    debouncedSearch.value = search.value;
    skipNextReload = state.value.kind === 'ready' ? 1 : 2;
    if (state.value.kind !== 'ready') {
      await new Promise<void>((resolve) => {
        const stop = watch(state, (s) => {
          if (s.kind === 'ready') {
            stop();
            resolve();
          }
        }, { immediate: true });
      });
    }
    await nextTick();
    reload();
    for (let i = 1; i < preservedNumPages.value; i++) {
      if (!hasMore.value) break;
      await loadMore();
    }
    await nextTick();
    setTimeout(() => {
      window.scrollTo(0, preservedScrollY.value);
    }, 50);
  } else {
    reload();
  }
  window.addEventListener(DB_CHANGE_EVENT, onChange);
  searchInput.value?.focus();
});

onBeforeUnmount(() => {
  if (debounceTimer !== null) clearTimeout(debounceTimer);
  window.removeEventListener(DB_CHANGE_EVENT, onChange);
});

function onFab(): void {
  router.push(`/entries/${uuidv7()}`);
}

function onRowClick(id: string): void {
  router.push(`/entries/${id}`);
}

async function onLogout(): Promise<void> {
  await logout();
}

function onRetry(): void {
  void syncNow();
}

const stateLabel = computed(() => {
  if (syncState.value.syncing) return 'Syncing…';
  if (syncState.value.lastError) return `Error: ${syncState.value.lastError}`;
  if (!syncState.value.online) return 'Offline';
  return 'Online';
});

const stateLabelClass = computed(() => {
  if (syncState.value.syncing) return 'sync-state-active';
  if (syncState.value.lastError) return 'sync-state-error';
  if (!syncState.value.online) return 'sync-state-offline';
  return 'sync-state-online';
});

function rowDotClass(entry: Entry): string {
  if (syncState.value.syncing) return 'row-dot row-dot-spinner';
  if (entry.local_dirty_at) return 'row-dot row-dot-dirty';
  return 'row-dot row-dot-synced';
}
</script>

<template>
  <main class="home">
    <header class="home-header">
      <h1 class="home-title">Linkomat</h1>
      <input
        ref="searchInput"
        v-model="search"
        type="search"
        placeholder="Search title, url, note, tags…"
        class="search"
        autocomplete="off"
        spellcheck="false"
        data-test="home-search"
      />
      <div class="sync-status" data-test="home-status-chip">
        <span
          v-if="syncState.pendingCount > 0"
          class="sync-status-pending"
          data-test="home-status-pending"
        >
          pending: {{ syncState.pendingCount }}
        </span>
        <span
          class="sync-status-state"
          :class="stateLabelClass"
          data-test="home-status-state"
        >{{ stateLabel }}</span>
        <button
          v-if="syncState.persistentFailure"
          type="button"
          class="sync-status-retry"
          data-test="home-status-retry"
          @click="onRetry"
        >
          Retry now
        </button>
      </div>
      <div class="home-meta">
        <span class="home-username" data-test="home-username">{{ username }}</span>
        <button
          type="button"
          class="logout-btn"
          data-test="home-logout"
          @click="onLogout"
        >
          Logout
        </button>
      </div>
    </header>

    <section v-if="state.kind === 'pending'" class="banner" data-test="home-status">
      Loading local database…
    </section>

    <section
      v-else-if="state.kind === 'opfs-unavailable'"
      class="banner banner-error"
      data-test="home-status"
    >
      Local storage unavailable. This browser does not expose OPFS.
    </section>

    <section
      v-else-if="state.kind === 'error'"
      class="banner banner-error"
      data-test="home-status"
    >
      {{ state.message }}
    </section>

    <section v-else data-test="home-results">
      <p v-if="entries.length === 0" class="empty" data-test="home-empty">
        {{ debouncedSearch.trim() ? 'No matches.' : 'No entries yet.' }}
      </p>
      <template v-else>
        <div class="list" data-test="home-list">
          <div
            v-for="item in entries"
            :key="item.id"
            class="row"
            data-test="home-row"
            :data-test-id="item.id"
            @click="onRowClick(item.id)"
          >
            <span class="row-status" :class="rowDotClass(item)" data-test="home-row-status" aria-hidden="true" />
            <div class="row-title">{{ item.title || item.url || '(untitled)' }}</div>
            <a
              v-if="item.url"
              class="row-url"
              :href="normalizeUrl(item.url)"
              target="_blank"
              rel="noopener noreferrer"
              data-test="home-row-url"
              @click.stop
            >{{ item.url }}</a>
            <span v-else class="row-url" />
            <div v-if="item.tags.length" class="row-tags">
              <span v-for="tag in item.tags" :key="tag" class="tag-chip">{{ tag }}</span>
            </div>
          </div>
        </div>
        <button
          v-if="hasMore"
          type="button"
          class="load-more"
          :disabled="isLoadingMore"
          data-test="home-load-more"
          @click="loadMore"
        >
          {{ isLoadingMore ? 'Loading…' : 'Load more' }}
        </button>
      </template>
    </section>

    <button
      type="button"
      class="fab"
      aria-label="Add entry"
      data-test="home-fab"
      @click="onFab"
    >
      +
    </button>
  </main>
</template>

<style scoped>
.home {
  max-width: 720px;
  margin: 0 auto;
  padding: 1rem;
  padding-bottom: 5rem;
}

.home-header {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.home-title {
  margin: 0;
  font-size: 1.5rem;
}

.search {
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.5rem;
  background: #ffffff;
  color: #111;
}

.search:focus {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.home-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: #6b7280;
}

.home-username {
  font-weight: 500;
}

.logout-btn {
  background: none;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  color: #374151;
}

.logout-btn:hover {
  background: #f3f4f6;
}

.sync-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  flex-wrap: wrap;
}

.sync-status-pending {
  background: #fef3c7;
  color: #92400e;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  font-weight: 500;
}

.sync-status-state {
  font-weight: 500;
}

.sync-state-online {
  color: #047857;
}

.sync-state-offline {
  color: #6b7280;
}

.sync-state-active {
  color: #2563eb;
}

.sync-state-error {
  color: #b91c1c;
}

.sync-status-retry {
  background: #2563eb;
  border: 1px solid #2563eb;
  border-radius: 0.375rem;
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  color: #ffffff;
}

.sync-status-retry:hover {
  background: #1d4ed8;
  border-color: #1d4ed8;
}

.list {
  border-top: 1px solid #e5e7eb;
}

.load-more {
  display: block;
  width: 100%;
  margin-top: 0.5rem;
  padding: 0.625rem 1rem;
  font-size: 0.875rem;
  background: #ffffff;
  color: #2563eb;
  border: 1px solid #d1d5db;
  border-radius: 0.5rem;
  cursor: pointer;
}

.load-more:hover:not(:disabled) {
  background: #f3f4f6;
}

.load-more:disabled {
  color: #6b7280;
  cursor: not-allowed;
}

.row {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.75rem 1rem;
  padding-right: 1.75rem;
  border-bottom: 1px solid #e5e7eb;
  height: 96px;
  overflow: hidden;
  background: #ffffff;
  box-sizing: border-box;
  cursor: pointer;
}

.row-status {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  pointer-events: none;
}

.row-dot {
  display: inline-block;
  width: 0.625rem;
  height: 0.625rem;
  border-radius: 50%;
}

.row-dot-synced {
  background: #10b981;
}

.row-dot-dirty {
  background: #f59e0b;
}

.row-dot-spinner {
  width: 0.75rem;
  height: 0.75rem;
  border: 2px solid #93c5fd;
  border-top-color: #2563eb;
  border-radius: 50%;
  box-sizing: border-box;
  animation: sync-spin 1s linear infinite;
}

@keyframes sync-spin {
  to {
    transform: rotate(360deg);
  }
}

.row-title {
  font-size: 1rem;
  font-weight: 500;
  color: #111;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.row-url {
  font-size: 0.875rem;
  color: #2563eb;
  text-decoration: underline;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.row-tags {
  display: flex;
  gap: 0.25rem;
  flex-wrap: nowrap;
  overflow: hidden;
}

.tag-chip {
  font-size: 0.75rem;
  padding: 0.125rem 0.5rem;
  background: #f3f4f6;
  color: #374151;
  border-radius: 9999px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.empty {
  color: #6b7280;
  text-align: center;
  padding: 2rem 0;
}

.banner {
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  background: #f3f4f6;
  color: #374151;
}

.banner-error {
  background: #fee2e2;
  color: #7f1d1d;
}

.fab {
  position: fixed;
  bottom: 1.5rem;
  right: 1.5rem;
  width: 3.5rem;
  height: 3.5rem;
  border: none;
  border-radius: 50%;
  background: #2563eb;
  color: #ffffff;
  font-size: 1.75rem;
  line-height: 1;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.fab:hover {
  background: #1d4ed8;
}
</style>