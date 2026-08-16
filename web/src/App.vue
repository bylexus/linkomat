<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuth } from './state/auth';

const route = useRoute();
const router = useRouter();
const { state, username, probe } = useAuth();

const showLoading = computed(() => state.value === 'unknown');

watch(
  [state, () => route.path],
  ([s, p]) => {
    if (s === 'logged-out' && p !== '/login') {
      void router.push('/login');
    } else if (s === 'logged-in' && p === '/login') {
      void router.push('/');
    }
  },
  { immediate: true }
);

onMounted(() => {
  if (state.value === 'unknown') {
    void probe();
  }
});

void username;
</script>

<template>
  <div v-if="showLoading" class="loading" data-test="app-loading">
    Loading…
  </div>
  <RouterView v-else />
</template>

<style>
:root {
  color-scheme: light;
  font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
  background: #ffffff;
  color: #111;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
}

button {
  font-family: inherit;
}

input {
  font-family: inherit;
}

.loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  color: #6b7280;
}
</style>
