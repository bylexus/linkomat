import { ref, computed, type Ref, type ComputedRef } from 'vue';
import { login as apiLogin, logout as apiLogout, probe as apiProbe } from '../api/auth';
import { setLogoutHandler } from '../api/client';
import { closeDb } from '../db';
import { clearPreservedHomeState } from './homeState';

export const USERNAME_STORAGE_KEY = 'linkomat.current_username';

export type AuthState = 'unknown' | 'logged-in' | 'logged-out';

const state: Ref<AuthState> = ref(determineInitialState());
const username: Ref<string | null> = ref(readStoredUsername());

function determineInitialState(): AuthState {
  return readStoredUsername() ? 'unknown' : 'logged-out';
}

function readStoredUsername(): string | null {
  if (typeof localStorage === 'undefined') return null;
  const v = localStorage.getItem(USERNAME_STORAGE_KEY);
  return v && v.length > 0 ? v : null;
}

function writeStoredUsername(value: string | null): void {
  if (typeof localStorage === 'undefined') return;
  if (value) {
    localStorage.setItem(USERNAME_STORAGE_KEY, value);
  } else {
    localStorage.removeItem(USERNAME_STORAGE_KEY);
  }
}

function markLoggedOut(): void {
  username.value = null;
  writeStoredUsername(null);
  closeDb();
  clearPreservedHomeState();
  state.value = 'logged-out';
}

setLogoutHandler(markLoggedOut);

export function useAuth(): {
  state: ComputedRef<AuthState>;
  username: ComputedRef<string | null>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  probe: () => Promise<void>;
  markLoggedOut: () => void;
} {
  return {
    state: computed(() => state.value),
    username: computed(() => username.value),
    login,
    logout,
    probe,
    markLoggedOut,
  };
}

async function login(user: string, password: string): Promise<void> {
  const res = await apiLogin(user, password);
  username.value = res.username;
  writeStoredUsername(res.username);
  state.value = 'logged-in';
}

async function logout(): Promise<void> {
  try {
    await apiLogout();
  } catch {
    // ignore — server may be unreachable
  }
  markLoggedOut();
}

async function probe(): Promise<void> {
  try {
    const ok = await apiProbe();
    if (ok) {
      state.value = 'logged-in';
    } else {
      markLoggedOut();
    }
  } catch {
    markLoggedOut();
  }
}
