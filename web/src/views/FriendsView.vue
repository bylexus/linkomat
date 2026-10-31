<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useAuth } from '../state/auth';
import type { ApiResult } from '../api/client';
import {
  listFriendships,
  createFriendship,
  acceptFriendship,
  declineFriendship,
  deleteFriendship,
  errorTitle,
  type Friendship,
} from '../api/friendships';

type RowKind = 'accepted' | 'outgoing' | 'incoming';
type Row = { friendship: Friendship; other: string; kind: RowKind };

const { username } = useAuth();

const friendships = ref<Friendship[]>([]);
const loading = ref(true);
const loadError = ref<string | null>(null);
const busyId = ref<string | null>(null);
const actionError = ref<{ id: string; message: string } | null>(null);

const dialogEl = ref<HTMLDialogElement | null>(null);
const newUsername = ref('');
const sending = ref(false);
const dialogError = ref<string | null>(null);

const rows = computed<Row[]>(() => {
  const me = username.value;
  const out: Row[] = [];
  for (const f of friendships.value) {
    if (f.status === 'accepted') {
      out.push({ friendship: f, other: f.requested_by === me ? f.accepted_by : f.requested_by, kind: 'accepted' });
    } else if (f.status === 'pending' && f.requested_by === me) {
      out.push({ friendship: f, other: f.accepted_by, kind: 'outgoing' });
    } else if (f.status === 'pending' && f.accepted_by === me) {
      out.push({ friendship: f, other: f.requested_by, kind: 'incoming' });
    }
  }
  return out;
});

const rowLabels: Record<RowKind, string> = {
  accepted: 'accepted',
  outgoing: 'requested by you',
  incoming: 'wants to be friends',
};

function messageFor(res: ApiResult<unknown>, offline: string, fallback: string): string {
  if (res.kind === 'network-error') return offline;
  if (res.kind === 'http-error') return errorTitle(res.body) ?? fallback;
  return fallback;
}

async function load(): Promise<void> {
  const res = await listFriendships();
  loading.value = false;
  if (res.kind === 'ok') {
    friendships.value = res.body.friendships ?? [];
    loadError.value = null;
    return;
  }
  loadError.value = messageFor(res, 'Friends are not available offline.', 'Could not load friends.');
}

async function runAction(id: string, action: () => Promise<ApiResult<unknown>>): Promise<void> {
  if (busyId.value !== null) return;
  busyId.value = id;
  actionError.value = null;
  try {
    const res = await action();
    if (res.kind === 'ok') {
      await load();
    } else {
      actionError.value = { id, message: messageFor(res, 'Not available offline.', 'Request failed.') };
    }
  } finally {
    busyId.value = null;
  }
}

function onRemove(row: Row): void {
  if (!window.confirm(`Remove ${row.other} as friend?`)) return;
  void runAction(row.friendship.id, () => deleteFriendship(row.friendship.id));
}

function onWithdraw(row: Row): void {
  void runAction(row.friendship.id, () => deleteFriendship(row.friendship.id));
}

function onAccept(row: Row): void {
  void runAction(row.friendship.id, () => acceptFriendship(row.friendship.id));
}

function onDecline(row: Row): void {
  void runAction(row.friendship.id, () => declineFriendship(row.friendship.id));
}

function openDialog(): void {
  newUsername.value = '';
  dialogError.value = null;
  dialogEl.value?.showModal();
}

function closeDialog(): void {
  dialogEl.value?.close();
}

async function onSend(): Promise<void> {
  const name = newUsername.value.trim();
  if (!name || sending.value) return;
  sending.value = true;
  dialogError.value = null;
  try {
    const res = await createFriendship(name);
    if (res.kind === 'ok') {
      closeDialog();
      newUsername.value = '';
      await load();
    } else {
      dialogError.value = messageFor(res, 'Not available offline.', 'Request failed.');
    }
  } finally {
    sending.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <main class="friends">
    <header class="friends-header">
      <RouterLink to="/" class="back-btn" aria-label="Back" data-test="friends-back">←</RouterLink>
      <h1 class="friends-title">Friends</h1>
    </header>

    <p v-if="loading" class="empty" data-test="friends-loading">Loading…</p>

    <section v-else-if="loadError" class="banner banner-error" data-test="friends-error">
      {{ loadError }}
    </section>

    <p v-else-if="rows.length === 0" class="empty" data-test="friends-empty">No friends yet.</p>

    <div v-else class="list" data-test="friends-list">
      <div v-for="row in rows" :key="row.friendship.id" class="row" data-test="friends-row"
        :data-test-id="row.friendship.id">
        <div class="row-main">
          <div class="row-info">
            <span class="row-name">{{ row.other }}</span>
            <span class="row-label">{{ rowLabels[row.kind] }}</span>
          </div>
          <div class="row-actions">
            <template v-if="row.kind === 'accepted'">
              <button type="button" class="btn btn-danger" :disabled="busyId !== null" data-test="friends-remove"
                @click="onRemove(row)">Remove</button>
            </template>
            <template v-else-if="row.kind === 'outgoing'">
              <button type="button" class="btn" :disabled="busyId !== null" data-test="friends-withdraw"
                @click="onWithdraw(row)">Withdraw</button>
            </template>
            <template v-else>
              <button type="button" class="btn btn-primary" :disabled="busyId !== null" data-test="friends-accept"
                @click="onAccept(row)">Accept</button>
              <button type="button" class="btn btn-danger" :disabled="busyId !== null" data-test="friends-decline"
                @click="onDecline(row)">Decline</button>
            </template>
          </div>
        </div>
        <p v-if="actionError && actionError.id === row.friendship.id" class="error" data-test="friends-row-error">
          {{ actionError.message }}
        </p>
      </div>
    </div>

    <button type="button" class="fab" aria-label="Add friend" data-test="friends-fab" @click="openDialog">
      +
    </button>

    <dialog ref="dialogEl" class="add-dialog" data-test="friends-dialog">
      <form class="add-form" @submit.prevent="onSend">
        <label class="add-label" for="friends-username">Username</label>
        <input id="friends-username" v-model="newUsername" type="text" class="add-input" autocomplete="off"
          autocapitalize="off" spellcheck="false" autofocus data-test="friends-username" />
        <p v-if="dialogError" class="error" data-test="friends-dialog-error">{{ dialogError }}</p>
        <div class="add-actions">
          <button type="button" class="btn" data-test="friends-cancel" @click="closeDialog">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="sending || !newUsername.trim()"
            data-test="friends-send">Send</button>
        </div>
      </form>
    </dialog>
  </main>
</template>

<style scoped>
.friends {
  max-width: 720px;
  margin: 0 auto;
  padding: 1rem;
  padding-bottom: 5rem;
}

.friends-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.friends-title {
  margin: 0;
  font-size: 1.5rem;
}

.back-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.25rem 0.75rem;
  font-size: 0.875rem;
  color: #374151;
  text-decoration: none;
}

.back-btn:hover {
  background: #f3f4f6;
}

.list {
  border-top: 1px solid #e5e7eb;
}

.row {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #e5e7eb;
}

.row-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.row-info {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  min-width: 0;
}

.row-name {
  font-size: 1rem;
  font-weight: 500;
  color: #111;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.row-label {
  font-size: 0.875rem;
  color: #6b7280;
}

.row-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
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

.btn-danger {
  color: #b91c1c;
}

.error {
  color: #b91c1c;
  font-size: 0.875rem;
  margin: 0.5rem 0 0 0;
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

.add-dialog {
  width: min(24rem, calc(100vw - 2rem));
  border: 1px solid #e5e7eb;
  border-radius: 0.5rem;
  padding: 1rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.add-dialog::backdrop {
  background: rgba(0, 0, 0, 0.3);
}

.add-form {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.add-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #374151;
}

.add-input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.5rem;
  background: #ffffff;
  color: #111;
}

.add-input:focus {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.add-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.add-dialog .error {
  margin: 0;
}
</style>
