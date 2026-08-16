import { apiGet, apiPut, apiDelete, type ApiResult } from './client';

export type ServerEntry = {
  id: string;
  title: string;
  url: string;
  note: string;
  tags: string[];
  created_at: string;
  updated_at: string;
  deleted_at: string | null | undefined;
};

export type EntriesListResponse = {
  entries: ServerEntry[];
  server_time: string;
};

export type EntryBody = {
  title: string;
  url: string;
  note: string;
  tags: string[];
};

export async function listEntries(
  params: { limit?: number; offset?: number; changed_since?: string } = {}
): Promise<ApiResult<EntriesListResponse>> {
  const qs = new URLSearchParams();
  if (params.limit !== undefined) qs.set('limit', String(params.limit));
  if (params.offset !== undefined) qs.set('offset', String(params.offset));
  if (params.changed_since !== undefined) qs.set('changed_since', params.changed_since);
  const path = qs.toString() ? `/entries?${qs.toString()}` : '/entries';
  return apiGet<EntriesListResponse>(path);
}

export async function getEntry(id: string): Promise<ApiResult<ServerEntry>> {
  return apiGet<ServerEntry>(`/entries/${encodeURIComponent(id)}`);
}

export async function putEntry(
  id: string,
  body: EntryBody
): Promise<ApiResult<ServerEntry>> {
  return apiPut<ServerEntry>(`/entries/${encodeURIComponent(id)}`, body);
}

export async function deleteEntry(id: string): Promise<ApiResult<null>> {
  return apiDelete<null>(`/entries/${encodeURIComponent(id)}`);
}
