<script setup lang="ts">
import { computed, ref } from 'vue';
import { useAuth } from '../state/auth';
import { useDb } from '../db';
import type { Entry } from '../db/sqlite';
import { shareEntry } from '../api/entries';
import { listFriendships, errorTitle } from '../api/friendships';

const { username } = useAuth();

const dialogEl = ref<HTMLDialogElement | null>(null);
const entry = ref<Entry | null>(null);
const friends = ref<string[]>([]);
const loading = ref(false);
const loadError = ref<string | null>(null);
const selected = ref('');
const sending = ref(false);
const sendError = ref<string | null>(null);
let openSeq = 0;

const heading = computed(() => {
  const e = entry.value;
  return `Share "${e ? e.title || e.url || '(untitled)' : ''}"`;
});

async function loadFriends(seq: number): Promise<void> {
  const res = await listFriendships();
  if (seq !== openSeq) return;
  loading.value = false;
  if (res.kind === 'ok') {
    const me = username.value;
    const names = new Set<string>();
    for (const f of res.body.friendships ?? []) {
      if (f.status !== 'accepted') continue;
      names.add(f.requested_by === me ? f.accepted_by : f.requested_by);
    }
    friends.value = [...names].sort((a, b) => a.localeCompare(b));
    return;
  }
  loadError.value =
    res.kind === 'network-error' ? 'Not available offline.' : errorTitle(res.body) ?? 'Could not load friends.';
}

function open(e: Entry): void {
  const seq = ++openSeq;
  entry.value = e;
  friends.value = [];
  selected.value = '';
  loading.value = true;
  loadError.value = null;
  sending.value = false;
  sendError.value = null;
  dialogEl.value?.showModal();
  void loadFriends(seq);
}

function close(): void {
  dialogEl.value?.close();
}

async function onSend(): Promise<void> {
  const current = entry.value;
  const friend = selected.value;
  if (!current || !friend || sending.value) return;
  const seq = openSeq;
  const { db } = useDb(username.value);
  const fresh = db ? db.getEntry(current.id) : null;
  if (fresh?.local_dirty_at) {
    sendError.value = 'Entry is not synced yet.';
    return;
  }
  sending.value = true;
  sendError.value = null;
  try {
    const res = await shareEntry(current.id, friend);
    if (res.kind === 'ok') {
      if (db) await db.applyServerEntry(res.body);
      if (seq === openSeq) close();
    } else if (seq === openSeq) {
      sendError.value =
        res.kind === 'network-error' ? 'Not available offline.' : errorTitle(res.body) ?? 'Request failed.';
    }
  } finally {
    if (seq === openSeq) sending.value = false;
  }
}

defineExpose({ open });
</script>

<template>
  <dialog ref="dialogEl" class="share-dialog" data-test="share-dialog">
    <form class="share-form" @submit.prevent="onSend">
      <h2 class="share-title" data-test="share-title">{{ heading }}</h2>

      <p v-if="loading" class="share-empty" data-test="share-loading">Loading…</p>

      <p v-else-if="loadError" class="error" data-test="share-load-error">{{ loadError }}</p>

      <p v-else-if="friends.length === 0" class="share-empty" data-test="share-no-friends">
        No friends yet.
        <RouterLink to="/friends" class="share-link" data-test="share-friends-link" @click="close">Add friends</RouterLink>
      </p>

      <div v-else class="share-list" role="radiogroup" aria-label="Friend" data-test="share-friends">
        <label v-for="name in friends" :key="name" class="share-option" data-test="share-friend">
          <input v-model="selected" type="radio" name="share-friend" :value="name" :disabled="sending" />
          <span class="share-name">{{ name }}</span>
        </label>
      </div>

      <p v-if="sendError" class="error" data-test="share-error">{{ sendError }}</p>

      <div class="share-actions">
        <button type="button" class="btn" data-test="share-cancel" @click="close">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="sending || !selected" data-test="share-send">
          Send
        </button>
      </div>
    </form>
  </dialog>
</template>

<style scoped>
.share-dialog {
  width: min(24rem, calc(100vw - 2rem));
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  padding: 1rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  cursor: default;
}

.share-dialog::backdrop {
  background: rgba(0, 0, 0, 0.3);
}

.share-form {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.share-title {
  margin: 0 0 0.25rem 0;
  font-size: 1rem;
  font-weight: 600;
  color: #111;
  overflow-wrap: anywhere;
}

.share-list {
  display: flex;
  flex-direction: column;
  max-height: 50vh;
  overflow-y: auto;
  border-top: 1px solid #e5e7eb;
}

.share-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.25rem;
  border-bottom: 1px solid #e5e7eb;
  cursor: pointer;
  font-size: 1rem;
  color: #111;
}

.share-option:hover {
  background: #f3f4f6;
}

.share-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.share-empty {
  color: #6b7280;
  font-size: 0.875rem;
  margin: 0;
  padding: 0.5rem 0;
}

.share-link {
  color: #2563eb;
}

.btn {
  background: none;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  color: #374151;
}

.btn:hover:not(:disabled) {
  background: #f3f4f6;
}

.btn:disabled {
  color: #9ca3af;
  cursor: not-allowed;
}

.btn-primary {
  background: #2563eb;
  border-color: #2563eb;
  color: #ffffff;
}

.btn-primary:hover:not(:disabled) {
  background: #1d4ed8;
  border-color: #1d4ed8;
}

.btn-primary:disabled {
  background: #93c5fd;
  border-color: #93c5fd;
  color: #ffffff;
}

.error {
  color: #b91c1c;
  font-size: 0.875rem;
  margin: 0;
}

.share-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
</style>
