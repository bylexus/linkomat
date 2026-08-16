import { ref, type Ref } from 'vue';

const ONLINE_POLL_INTERVAL_MS = 10_000;
const HEALTHZ_TIMEOUT_MS = 5_000;

const online: Ref<boolean> = ref(
  typeof navigator !== 'undefined' ? navigator.onLine : false
);

let pollTimer: number | null = null;
let started = false;

async function probeServer(): Promise<boolean> {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), HEALTHZ_TIMEOUT_MS);
    const res = await fetch('/healthz', {
      method: 'GET',
      credentials: 'same-origin',
      signal: controller.signal,
    });
    clearTimeout(timeout);
    return res.ok;
  } catch {
    return false;
  }
}

async function recompute(): Promise<void> {
  const serverUp = await probeServer();
  online.value = (typeof navigator !== 'undefined' ? navigator.onLine : false) && serverUp;
}

function onWindowOnline(): void {
  void recompute();
}

function onWindowOffline(): void {
  online.value = false;
}

export function startReachability(): void {
  if (started || typeof window === 'undefined') return;
  started = true;
  window.addEventListener('online', onWindowOnline);
  window.addEventListener('offline', onWindowOffline);
  pollTimer = window.setInterval(() => {
    void recompute();
  }, ONLINE_POLL_INTERVAL_MS);
  void recompute();
}

export function stopReachability(): void {
  if (typeof window === 'undefined') return;
  if (pollTimer !== null) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  window.removeEventListener('online', onWindowOnline);
  window.removeEventListener('offline', onWindowOffline);
  started = false;
}

export function useReachability(): Ref<boolean> {
  return online;
}

export async function refreshReachability(): Promise<void> {
  await recompute();
}
