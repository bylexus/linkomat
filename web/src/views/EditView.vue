<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useDb } from '../db';
import type { Entry } from '../db/sqlite';
import { normalizeUrl } from '../utils/url';
import { useAuth } from '../state/auth';

const route = useRoute();
const router = useRouter();
const { username } = useAuth();
const { state, db } = useDb(username.value);

const id = computed(() => String(route.params.id));

const loaded = ref<Entry | null>(null);
const form = reactive({
  title: '',
  url: '',
  note: '',
  tags: [] as string[],
});
const tagInput = ref('');
const error = ref<string | null>(null);

onMounted(() => {
  if (!db || state.value.kind !== 'ready') return;
  const existing = db.getEntry(id.value);
  if (existing) {
    loaded.value = existing;
    form.title = existing.title;
    form.url = existing.url;
    form.note = existing.note;
    form.tags = [...existing.tags];
  }
});

function isFormEmpty(): boolean {
  return (
    !loaded.value &&
    form.title === '' &&
    form.url === '' &&
    form.note === '' &&
    form.tags.length === 0
  );
}

async function saveField(): Promise<void> {
  if (!db || state.value.kind !== 'ready') return;
  if (isFormEmpty()) return;
  try {
    const result = await db.upsertEntry({
      id: id.value,
      title: form.title,
      url: form.url,
      note: form.note,
      tags: form.tags,
    });
    loaded.value = result;
    error.value = null;
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

async function onUrlBlur(): Promise<void> {
  const normalized = normalizeUrl(form.url);
  if (normalized !== form.url) {
    form.url = normalized;
  }
  await saveField();
}

function addTag(): void {
  const v = tagInput.value.trim().replace(/,$/, '').trim();
  if (!v) return;
  if (!form.tags.includes(v)) {
    form.tags = [...form.tags, v];
  }
  tagInput.value = '';
  void saveField();
}

function removeTag(tag: string): void {
  form.tags = form.tags.filter((t) => t !== tag);
  void saveField();
}

async function onDelete(): Promise<void> {
  if (!db || state.value.kind !== 'ready') return;
  if (!loaded.value) {
    router.push('/');
    return;
  }
  if (!window.confirm('Delete this entry?')) return;
  try {
    await db.softDeleteEntry(id.value);
    router.push('/');
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

function goBack(): void {
  router.push('/');
}
</script>

<template>
  <main class="edit">
    <header class="edit-header">
      <button type="button" class="icon-btn" @click="goBack" data-test="edit-back">←</button>
      <span class="edit-title">{{ loaded ? 'Edit entry' : 'New entry' }}</span>
      <button
        v-if="loaded"
        type="button"
        class="icon-btn danger"
        @click="onDelete"
        data-test="edit-delete"
      >
        Delete
      </button>
      <span v-else class="icon-btn-spacer" />
    </header>

    <section v-if="state.kind !== 'ready'" class="banner" data-test="edit-status">
      <span v-if="state.kind === 'pending'">Loading local database…</span>
      <span v-else-if="state.kind === 'opfs-unavailable'">
        Local storage unavailable. This browser does not expose OPFS.
      </span>
      <span v-else>{{ state.message }}</span>
    </section>

    <section v-else class="edit-form" data-test="edit-form">
      <label class="field">
        <span class="field-label">Title</span>
        <input
          v-model="form.title"
          type="text"
          autocomplete="off"
          spellcheck="false"
          data-test="edit-title"
          @blur="saveField"
        />
      </label>

      <label class="field">
        <span class="field-label">URL</span>
        <input
          v-model="form.url"
          type="url"
          autocomplete="off"
          spellcheck="false"
          inputmode="url"
          data-test="edit-url"
          @blur="onUrlBlur"
        />
      </label>

      <label class="field">
        <span class="field-label">Tags</span>
        <div class="tags" data-test="edit-tags">
          <span v-for="tag in form.tags" :key="tag" class="tag-chip">
            {{ tag }}
            <button
              type="button"
              class="tag-remove"
              :aria-label="`Remove tag ${tag}`"
              :data-test="`edit-tag-remove-${tag}`"
              @click="removeTag(tag)"
            >
              ×
            </button>
          </span>
          <input
            v-model="tagInput"
            type="text"
            class="tag-input"
            placeholder="Add tag, press Enter"
            autocomplete="off"
            spellcheck="false"
            data-test="edit-tag-input"
            @keydown.enter.prevent="addTag"
            @keydown.,.prevent="addTag"
            @blur="addTag"
          />
        </div>
      </label>

      <label class="field">
        <span class="field-label">Note</span>
        <textarea
          v-model="form.note"
          rows="12"
          data-test="edit-note"
          @blur="saveField"
        />
      </label>

      <p v-if="error" class="error" data-test="edit-error">{{ error }}</p>
    </section>
  </main>
</template>

<style scoped>
.edit {
  max-width: 720px;
  margin: 0 auto;
  padding: 1rem;
}

.edit-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.edit-title {
  font-size: 1.125rem;
  font-weight: 500;
  flex: 1;
  text-align: center;
}

.icon-btn {
  background: none;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.375rem 0.75rem;
  cursor: pointer;
  font-size: 0.875rem;
  min-width: 4rem;
}

.icon-btn:hover {
  background: #f3f4f6;
}

.icon-btn.danger {
  color: #b91c1c;
}

.icon-btn-spacer {
  min-width: 4rem;
}

.edit-form {
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

.field input,
.field textarea {
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #ffffff;
  color: #111;
  font-family: inherit;
}

.field input:focus,
.field textarea:focus {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.field textarea {
  resize: vertical;
  min-height: 8rem;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  align-items: center;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  padding: 0.5rem;
  min-height: 2.5rem;
}

.tags:focus-within {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  background: #f3f4f6;
  color: #374151;
  padding: 0.125rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.875rem;
}

.tag-remove {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  font-size: 1rem;
  line-height: 1;
  color: #6b7280;
}

.tag-input {
  flex: 1;
  min-width: 8rem;
  border: none;
  outline: none;
  padding: 0.125rem 0.25rem;
  font-size: 0.875rem;
  background: transparent;
}

.banner {
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  background: #f3f4f6;
  color: #374151;
}

.error {
  color: #b91c1c;
  font-size: 0.875rem;
  margin: 0;
}
</style>