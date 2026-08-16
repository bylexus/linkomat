<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useAuth } from '../state/auth';

const router = useRouter();
const { login } = useAuth();

const username = ref('');
const password = ref('');
const submitting = ref(false);
const error = ref<string | null>(null);

async function onSubmit(): Promise<void> {
  if (submitting.value) return;
  if (!username.value.trim() || !password.value) {
    error.value = 'Username and password are required.';
    return;
  }
  submitting.value = true;
  error.value = null;
  try {
    await login(username.value.trim(), password.value);
    password.value = '';
    await router.push('/');
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Login failed.';
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <main class="login">
    <h1 class="login-title">Linkomat</h1>
    <form class="login-form" data-test="login-form" @submit.prevent="onSubmit">
      <label class="field">
        <span class="field-label">Username</span>
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          spellcheck="false"
          autocapitalize="off"
          autocorrect="off"
          required
          data-test="login-username"
        />
      </label>
      <label class="field">
        <span class="field-label">Password</span>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          data-test="login-password"
        />
      </label>
      <button
        type="submit"
        class="submit"
        :disabled="submitting"
        data-test="login-submit"
      >
        {{ submitting ? 'Signing in…' : 'Sign in' }}
      </button>
      <p v-if="error" class="error" data-test="login-error">{{ error }}</p>
    </form>
  </main>
</template>

<style scoped>
.login {
  max-width: 20rem;
  margin: 4rem auto;
  padding: 1rem;
}

.login-title {
  margin: 0 0 1.5rem 0;
  font-size: 1.5rem;
  text-align: center;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.field-label {
  font-size: 0.875rem;
  color: #374151;
}

.field input {
  padding: 0.5rem 0.75rem;
  font-size: 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #ffffff;
  color: #111;
}

.field input:focus {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.submit {
  padding: 0.625rem 1rem;
  font-size: 1rem;
  background: #2563eb;
  color: #ffffff;
  border: none;
  border-radius: 0.375rem;
  cursor: pointer;
}

.submit:hover:not(:disabled) {
  background: #1d4ed8;
}

.submit:disabled {
  background: #93c5fd;
  cursor: not-allowed;
}

.error {
  color: #b91c1c;
  font-size: 0.875rem;
  margin: 0;
}
</style>
