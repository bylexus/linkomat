<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { RecycleScroller } from 'vue-virtual-scroller';
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css';
import { useDb } from '../db';
import { DB_CHANGE_EVENT } from '../db/sqlite';
import { uuidv7 } from '../utils/uuid';
import { normalizeUrl } from '../utils/url';
import type { Entry } from '../db/sqlite';

const { state, db } = useDb();
const router = useRouter();

const search = ref('');
const debouncedSearch = ref('');
const entries = ref<Entry[]>([]);

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
    return;
  }
  const q = debouncedSearch.value.trim();
  entries.value = q ? db.searchEntries(q) : db.listEntries();
}

watch([debouncedSearch, state], () => {
  reload();
});

function onChange(): void {
  reload();
}

onMounted(() => {
  reload();
  window.addEventListener(DB_CHANGE_EVENT, onChange);
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
</script>

<template>
  <main class="home">
    <header class="home-header">
      <h1 class="home-title">Linkomat</h1>
      <input
        v-model="search"
        type="search"
        placeholder="Search title, url, note, tags…"
        class="search"
        autocomplete="off"
        spellcheck="false"
        data-test="home-search"
      />
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
      <RecycleScroller
        v-else
        :items="entries"
        :item-size="96"
        key-field="id"
        class="list"
        data-test="home-list"
      >
        <template #default="{ item }">
          <div
            class="row"
            data-test="home-row"
            :data-test-id="item.id"
            @click="onRowClick(item.id)"
          >
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
        </template>
      </RecycleScroller>
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

.list {
  height: calc(100vh - 11rem);
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  overflow: hidden;
}

.row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #e5e7eb;
  height: 96px;
  overflow: hidden;
  background: #ffffff;
  box-sizing: border-box;
  cursor: pointer;
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