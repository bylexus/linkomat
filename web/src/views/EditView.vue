<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useDb } from '../db';
import { DB_CHANGE_EVENT, type Entry } from '../db/sqlite';
import { normalizeUrl } from '../utils/url';
import { useAuth } from '../state/auth';
import { useSyncState } from '../state/sync';
import { syncNow } from '../sync/engine';
import ShareIcon from '../components/icons/ShareIcon.vue';
import ShareDialog from '../components/ShareDialog.vue';

const route = useRoute();
const router = useRouter();
const { username } = useAuth();
const { state, db } = useDb(username.value);
const syncState = useSyncState();

const id = computed(() => String(route.params.id));

const loaded = ref<Entry | null>(null);
const form = reactive({
  title: '',
  url: '',
  note: '',
  tags: [] as string[],
});
const tagInput = ref('');
const error = ref<string | null>(null);

function reloadLoaded(): void {
  if (!db || state.value.kind !== 'ready') return;
  if (!loaded.value) return;
  const fresh = db.getEntry(id.value);
  if (fresh) {
    loaded.value = fresh;
  }
}

function onDbChange(): void {
  reloadLoaded();
}

let initialLoadDone = false;

function loadInitial(): void {
  if (initialLoadDone || !db || state.value.kind !== 'ready') return;
  initialLoadDone = true;
  const existing = db.getEntry(id.value);
  if (existing) {
    loaded.value = existing;
    form.title = existing.title;
    form.url = existing.url;
    form.note = existing.note;
    form.tags = [...existing.tags];
  }
}

watch(state, loadInitial);

onMounted(() => {
  loadInitial();
  window.addEventListener(DB_CHANGE_EVENT, onDbChange);
});

onBeforeUnmount(() => {
  window.removeEventListener(DB_CHANGE_EVENT, onDbChange);
});

function isFormEmpty(): boolean {
  return (
    !loaded.value &&
    form.title === '' &&
    form.url === '' &&
    form.note === '' &&
    form.tags.length === 0
  );
}

async function saveField(): Promise<void> {
  if (!db || state.value.kind !== 'ready') return;
  if (isFormEmpty()) return;
  try {
    const result = await db.upsertEntry({
      id: id.value,
      title: form.title,
      url: form.url,
      note: form.note,
      tags: form.tags,
    });
    loaded.value = result;
    error.value = null;
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

async function onUrlBlur(): Promise<void> {
  const normalized = normalizeUrl(form.url);
  if (normalized !== form.url) {
    form.url = normalized;
  }
  await saveField();
}

function addTag(): void {
  const v = tagInput.value.trim().replace(/,$/, '').trim();
  if (!v) return;
  if (!form.tags.includes(v)) {
    form.tags = [...form.tags, v];
  }
  tagInput.value = '';
  void saveField();
}

function removeTag(tag: string): void {
  form.tags = form.tags.filter((t) => t !== tag);
  void saveField();
}

async function onDelete(): Promise<void> {
  if (!db || state.value.kind !== 'ready') return;
  if (!loaded.value) {
    router.push('/');
    return;
  }
  if (!window.confirm('Delete this entry?')) return;
  try {
    await db.softDeleteEntry(id.value);
    router.push('/');
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

const shareDialog = ref<InstanceType<typeof ShareDialog> | null>(null);

const shareLabel = computed(() => {
  if (!syncState.value.online) return 'Offline';
  if (loaded.value?.local_dirty_at) return 'Sync first';
  return 'Share';
});

function onShare(): void {
  if (!loaded.value || shareLabel.value !== 'Share') return;
  shareDialog.value?.open(loaded.value);
}

const metaLines = computed<string[]>(() => {
  const meta = loaded.value?.meta;
  if (!meta) return [];
  const lines: string[] = [];
  for (const share of meta.shares ?? []) {
    lines.push(`Sent to ${share.to} on ${new Date(share.at).toLocaleString()}`);
  }
  if (meta.imported) {
    lines.push(`Imported from ${meta.imported.from} on ${new Date(meta.imported.at).toLocaleString()}`);
  }
  return lines;
});

function goBack(): void {
  router.push('/');
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

const entryDotClass = computed(() => {
  if (syncState.value.syncing) return 'edit-entry-dot edit-entry-dot-spinner';
  if (loaded.value?.local_dirty_at) return 'edit-entry-dot edit-entry-dot-dirty';
  return 'edit-entry-dot edit-entry-dot-synced';
});
</script>

<template>
  <main class="edit">
    <header class="edit-header">
      <button type="button" class="icon-btn" @click="goBack" data-test="edit-back">←</button>
      <div class="edit-title-area">
        <span class="edit-entry-dot" :class="entryDotClass" data-test="edit-entry-dot" aria-hidden="true" />
        <span class="edit-title">{{ loaded ? 'Edit entry' : 'New entry' }}</span>
      </div>
      <div v-if="loaded" class="edit-header-actions">
        <button
          type="button"
          class="icon-btn share-btn"
          :disabled="shareLabel !== 'Share'"
          :aria-label="shareLabel"
          :title="shareLabel"
          data-test="edit-share"
          @click="onShare"
        >
          <ShareIcon width="1rem" height="1rem" />
        </button>
        <button
          type="button"
          class="icon-btn danger"
          @click="onDelete"
          data-test="edit-delete"
        >
          Delete
        </button>
      </div>
      <span v-else class="icon-btn-spacer" />
    </header>

    <div class="edit-sync-status" data-test="edit-status-chip">
      <span
        class="sync-status-state"
        :class="stateLabelClass"
        data-test="edit-status-state"
      >{{ stateLabel }}</span>
      <span
        v-if="syncState.pendingCount > 0"
        class="sync-status-pending"
        data-test="edit-status-pending"
      >
        pending: {{ syncState.pendingCount }}
      </span>
      <button
        v-if="syncState.persistentFailure"
        type="button"
        class="sync-status-retry"
        data-test="edit-status-retry"
        @click="onRetry"
      >
        Retry now
      </button>
    </div>

    <section v-if="state.kind !== 'ready'" class="banner" data-test="edit-status">
      <span v-if="state.kind === 'pending'">Loading local database…</span>
      <span v-else-if="state.kind === 'opfs-unavailable'">
        Local storage unavailable. This browser does not expose OPFS.
      </span>
      <span v-else>{{ state.message }}</span>
    </section>

    <section v-else class="edit-form" data-test="edit-form">
      <label class="field">
        <span class="field-label">Title</span>
        <input
          v-model="form.title"
          type="text"
          autocomplete="off"
          spellcheck="false"
          data-test="edit-title"
          @blur="saveField"
        />
      </label>

      <label class="field">
        <span class="field-label">URL</span>
        <input
          v-model="form.url"
          type="url"
          autocomplete="off"
          spellcheck="false"
          inputmode="url"
          data-test="edit-url"
          @blur="onUrlBlur"
        />
      </label>

      <label class="field">
        <span class="field-label">Tags</span>
        <div class="tags" data-test="edit-tags">
          <span v-for="tag in form.tags" :key="tag" class="tag-chip">
            {{ tag }}
            <button
              type="button"
              class="tag-remove"
              :aria-label="`Remove tag ${tag}`"
              :data-test="`edit-tag-remove-${tag}`"
              @click="removeTag(tag)"
            >
              ×
            </button>
          </span>
          <input
            v-model="tagInput"
            type="text"
            class="tag-input"
            placeholder="Add tag, press Enter"
            autocomplete="off"
            spellcheck="false"
            data-test="edit-tag-input"
            @keydown.enter.prevent="addTag"
            @keydown.,.prevent="addTag"
            @blur="addTag"
          />
        </div>
      </label>

      <label class="field">
        <span class="field-label">Note</span>
        <textarea
          v-model="form.note"
          rows="12"
          data-test="edit-note"
          @blur="saveField"
        />
      </label>

      <p v-if="error" class="error" data-test="edit-error">{{ error }}</p>

      <footer v-if="metaLines.length > 0" class="edit-meta" data-test="edit-meta">
        <p v-for="(line, i) in metaLines" :key="i" class="edit-meta-line" data-test="edit-meta-line">{{ line }}</p>
      </footer>
    </section>

    <ShareDialog ref="shareDialog" />
  </main>
</template>

<style scoped>
.edit {
  max-width: 720px;
  margin: 0 auto;
  padding: 1rem;
}

.edit-header {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.edit-header > :first-child {
  justify-self: start;
}

.edit-header-actions,
.icon-btn-spacer {
  justify-self: end;
}

.edit-header-actions {
  display: flex;
  align-items: stretch;
  gap: 0.5rem;
}

.edit-title-area {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
}

.edit-title {
  font-size: 1.125rem;
  font-weight: 500;
}

.edit-entry-dot {
  display: inline-block;
  width: 0.625rem;
  height: 0.625rem;
  border-radius: 50%;
}

.edit-entry-dot-synced {
  background: #10b981;
}

.edit-entry-dot-dirty {
  background: #f59e0b;
}

.edit-entry-dot-spinner {
  width: 0.75rem;
  height: 0.75rem;
  border: 2px solid #93c5fd;
  border-top-color: #2563eb;
  border-radius: 50%;
  box-sizing: border-box;
  animation: edit-sync-spin 1s linear infinite;
}

@keyframes edit-sync-spin {
  to {
    transform: rotate(360deg);
  }
}

.edit-sync-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
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

.sync-status-pending {
  background: #fef3c7;
  color: #92400e;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  font-weight: 500;
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

.icon-btn {
  background: none;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.375rem 0.75rem;
  cursor: pointer;
  font-size: 0.875rem;
  min-width: 4rem;
}

.icon-btn:hover {
  background: #f3f4f6;
}

.icon-btn.danger {
  color: #b91c1c;
}

.icon-btn.share-btn {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.375rem 0.5rem;
  color: #374151;
}

.icon-btn:disabled {
  color: #9ca3af;
  background: none;
  cursor: not-allowed;
}

.icon-btn-spacer {
  min-width: 4rem;
}

.edit-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.field-label {
  font-size: 0.875rem;
  color: #374151;
}

.field input,
.field textarea {
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #ffffff;
  color: #111;
  font-family: inherit;
}

.field input:focus,
.field textarea:focus {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.field textarea {
  resize: vertical;
  min-height: 8rem;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  align-items: center;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.5rem;
  min-height: 2.5rem;
}

.tags:focus-within {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  background: #f3f4f6;
  color: #374151;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.875rem;
}

.tag-remove {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  font-size: 1rem;
  line-height: 1;
  color: #6b7280;
}

.tag-input {
  flex: 1;
  min-width: 8rem;
  border: none;
  outline: none;
  padding: 0.125rem 0.25rem;
  font-size: 0.875rem;
  background: transparent;
}

.banner {
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  background: #f3f4f6;
  color: #374151;
}

.error {
  color: #b91c1c;
  font-size: 0.875rem;
  margin: 0;
}

.edit-meta {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding-top: 0.75rem;
  border-top: 1px solid #e5e7eb;
}

.edit-meta-line {
  margin: 0;
  font-size: 0.875rem;
  color: #6b7280;
}
</style>