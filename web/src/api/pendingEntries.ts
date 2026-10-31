import { apiGet, apiPost, apiDelete, type ApiResult } from './client';
import type { ServerEntry } from './entries';

export type PendingEntry = {
  id: string;
  from: string;
  source_entry_id: string;
  title: string;
  url: string;
  note: string;
  tags: string[];
  shared_at: string;
};

export type PendingEntriesListResponse = {
  pending_entries: PendingEntry[];
};

export async function listPendingEntries(): Promise<ApiResult<PendingEntriesListResponse>> {
  return apiGet<PendingEntriesListResponse>('/pending-entries');
}

export async function acceptPendingEntry(id: string): Promise<ApiResult<ServerEntry>> {
  return apiPost<ServerEntry>(`/pending-entries/${encodeURIComponent(id)}/accept`);
}

export async function declinePendingEntry(id: string): Promise<ApiResult<null>> {
  return apiDelete<null>(`/pending-entries/${encodeURIComponent(id)}`);
}
