import { apiGet, apiPost, apiDelete, type ApiResult } from './client';

export type FriendshipStatus = 'pending' | 'accepted' | 'declined';

export type Friendship = {
  id: string;
  requested_by: string;
  accepted_by: string;
  status: FriendshipStatus;
  requested_at: string;
  accepted_at: string | null;
  declined_at: string | null;
};

export type FriendshipsListResponse = {
  friendships: Friendship[];
};

export async function listFriendships(): Promise<ApiResult<FriendshipsListResponse>> {
  return apiGet<FriendshipsListResponse>('/friendships');
}

export async function createFriendship(username: string): Promise<ApiResult<Friendship>> {
  return apiPost<Friendship>('/friendships', { username });
}

export async function acceptFriendship(id: string): Promise<ApiResult<Friendship>> {
  return apiPost<Friendship>(`/friendships/${encodeURIComponent(id)}/accept`);
}

export async function declineFriendship(id: string): Promise<ApiResult<Friendship>> {
  return apiPost<Friendship>(`/friendships/${encodeURIComponent(id)}/decline`);
}

export async function deleteFriendship(id: string): Promise<ApiResult<null>> {
  return apiDelete<null>(`/friendships/${encodeURIComponent(id)}`);
}

export function errorTitle(body: unknown): string | null {
  if (body && typeof body === 'object' && 'title' in body) {
    const title = (body as { title: unknown }).title;
    if (typeof title === 'string' && title.length > 0) return title;
  }
  return null;
}
