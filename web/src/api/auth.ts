import { apiGet, apiPost, type ApiResult } from './client';

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  username: string;
}

export async function login(
  username: string,
  password: string
): Promise<LoginResponse> {
  const res = await apiPost<LoginResponse>('/auth/login', { username, password });
  if (res.kind === 'ok') {
    return res.body;
  }
  if (res.kind === 'http-error' && res.status === 401) {
    throw new Error('Invalid username or password.');
  }
  if (res.kind === 'http-error' && res.status === 403) {
    throw new Error('Account is deactivated.');
  }
  if (res.kind === 'http-error' && res.status === 429) {
    throw new Error('Too many login attempts. Try again later.');
  }
  throw new Error('Login failed.');
}

export async function logout(): Promise<void> {
  const res = await apiPost<null>('/auth/logout');
  if (res.kind === 'network-error' || res.kind === 'http-error') {
    // Best-effort: even if the server is unreachable, log out locally.
  }
}

export async function probe(): Promise<boolean> {
  const res = await apiGet<unknown>('/entries?limit=1');
  if (res.kind === 'ok') return true;
  return false;
}
