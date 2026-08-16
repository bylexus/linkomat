import { ref, computed, type Ref, type ComputedRef } from 'vue';

export type SyncSkipReason =
  | 'in-flight'
  | 'no-username'
  | 'offline'
  | 'db-not-ready';

export type SyncState = {
  online: boolean;
  syncing: boolean;
  pendingCount: number;
  lastError: string | null;
  lastSkipReason: SyncSkipReason | null;
  lastSyncAt: string | null;
  persistentFailure: boolean;
};

const state: Ref<SyncState> = ref({
  online: false,
  syncing: false,
  pendingCount: 0,
  lastError: null,
  lastSkipReason: null,
  lastSyncAt: null,
  persistentFailure: false,
});

export function useSyncState(): ComputedRef<SyncState> {
  return computed(() => state.value);
}

export function setSyncState(patch: Partial<SyncState>): void {
  state.value = { ...state.value, ...patch };
}

export function syncStateRef(): Ref<SyncState> {
  return state;
}
