<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { LocalDb, type DbState } from './db/sqlite'

const state = ref<DbState>({ kind: 'pending' })

onMounted(async () => {
  const db = new LocalDb()
  state.value = await db.open()
  db.close()
})
</script>

<template>
  <main class="app">
    <h1>Linkomat</h1>

    <section v-if="state.kind === 'pending'" data-test="db-pending">
      <p>Loading local database…</p>
    </section>

    <section v-else-if="state.kind === 'opfs-unavailable'" class="banner banner-error" data-test="db-opfs-unavailable">
      <strong>Local storage unavailable.</strong>
      <p>
        This browser does not expose OPFS. The web client needs OPFS to store
        entries locally. Please use a recent Chromium-based browser or a
        PWA-capable context.
      </p>
    </section>

    <section v-else-if="state.kind === 'error'" class="banner banner-error" data-test="db-error">
      <strong>Local database error.</strong>
      <p>{{ state.message }}</p>
    </section>

    <section v-else class="status" data-test="db-status">
      <dl>
        <dt>Loaded</dt>
        <dd>yes</dd>
        <dt>Fresh (first launch)</dt>
        <dd>{{ state.fresh ? 'yes' : 'no' }}</dd>
        <dt>Integrity check</dt>
        <dd>{{ state.integrityOk ? 'ok' : 'failed (recovered)' }}</dd>
        <dt>Schema version</dt>
        <dd>{{ state.schemaVersion }}</dd>
      </dl>
      <p v-if="state.sidecarName" class="banner banner-warn" data-test="db-sidecar">
        Previous database preserved as <code>{{ state.sidecarName }}</code>.
      </p>
      <p v-if="state.errorMessage" class="banner banner-warn" data-test="db-recovery">
        {{ state.errorMessage }}
      </p>
    </section>
  </main>
</template>

<style>
:root {
  color-scheme: light;
  font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
  background: #ffffff;
  color: #111;
}

.app {
  max-width: 720px;
  margin: 2rem auto;
  padding: 0 1rem;
}

h1 {
  margin: 0 0 1rem;
}

dl {
  display: grid;
  grid-template-columns: max-content 1fr;
  column-gap: 1rem;
  row-gap: 0.25rem;
  margin: 0;
}

dt {
  font-weight: 600;
}

dd {
  margin: 0;
}

.banner {
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
}

.banner-error {
  background: #fee2e2;
  color: #7f1d1d;
}

.banner-warn {
  background: #fef3c7;
  color: #78350f;
}

code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
</style>