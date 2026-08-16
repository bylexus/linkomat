type LogoutHandler = () => void;
let logoutHandler: LogoutHandler = () => {};

export function setLogoutHandler(handler: LogoutHandler): void {
  logoutHandler = handler;
}

let refreshInFlight: Promise<boolean> | null = null;

async function tryRefresh(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = (async () => {
    try {
      const res = await fetch('/auth/refresh', {
        method: 'POST',
        credentials: 'same-origin',
      });
      return res.ok;
    } catch {
      return false;
    }
  })();
  try {
    return await refreshInFlight;
  } finally {
    refreshInFlight = null;
  }
}

export type ApiResult<T> =
  | { kind: 'ok'; status: number; body: T }
  | { kind: 'http-error'; status: number; body: unknown }
  | { kind: 'network-error'; error: string };

export async function apiRequest<T>(
  method: string,
  path: string,
  body?: unknown
): Promise<ApiResult<T>> {
  const init: RequestInit = {
    method,
    credentials: 'same-origin',
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  };
  let res: Response;
  try {
    res = await fetch(path, init);
  } catch (err) {
    return { kind: 'network-error', error: errorMessage(err) };
  }
  if (res.status === 401) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      try {
        res = await fetch(path, init);
      } catch (err) {
        return { kind: 'network-error', error: errorMessage(err) };
      }
    } else {
      logoutHandler();
      return { kind: 'http-error', status: 401, body: await safeJson(res) };
    }
  }
  const parsed = await safeJson(res);
  if (res.ok) {
    if (res.status === 204) {
      return { kind: 'ok', status: 204, body: undefined as T };
    }
    return { kind: 'ok', status: res.status, body: parsed as T };
  }
  return { kind: 'http-error', status: res.status, body: parsed };
}

export async function apiGet<T>(path: string): Promise<ApiResult<T>> {
  return apiRequest<T>('GET', path);
}

export async function apiPost<T>(path: string, body?: unknown): Promise<ApiResult<T>> {
  return apiRequest<T>('POST', path, body);
}

export async function apiPut<T>(path: string, body?: unknown): Promise<ApiResult<T>> {
  return apiRequest<T>('PUT', path, body);
}

export async function apiDelete<T>(path: string): Promise<ApiResult<T>> {
  return apiRequest<T>('DELETE', path);
}

async function safeJson(res: Response): Promise<unknown> {
  try {
    const text = await res.text();
    if (text.length === 0) return null;
    return JSON.parse(text);
  } catch {
    return null;
  }
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
