<script setup lang="ts">
import { computed, ref, useId } from 'vue';

const props = defineProps<{
  modelValue: string[];
  suggestionSource?: () => string[];
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string[]];
  change: [];
}>();

const tagInput = ref('');
const inputEl = ref<HTMLInputElement | null>(null);
const knownTags = ref<string[]>([]);
const focused = ref(false);
const dismissed = ref(false);
const highlight = ref(-1);

const listId = `tag-suggestions-${useId()}`;

const MAX_SUGGESTIONS = 8;

const TAG_SEPARATOR = /[,\s]+/;

const suggestions = computed<string[]>(() => {
  const q = tagInput.value.trim().toLocaleLowerCase();
  if (q.length < 1) return [];
  const prefix: string[] = [];
  const other: string[] = [];
  for (const tag of knownTags.value) {
    if (props.modelValue.includes(tag)) continue;
    const lower = tag.toLocaleLowerCase();
    if (!lower.includes(q)) continue;
    if (lower.startsWith(q)) prefix.push(tag);
    else other.push(tag);
  }
  prefix.sort((a, b) => a.localeCompare(b));
  other.sort((a, b) => a.localeCompare(b));
  return [...prefix, ...other].slice(0, MAX_SUGGESTIONS);
});

const listOpen = computed(
  () => focused.value && !dismissed.value && suggestions.value.length > 0,
);

const activeDescendant = computed(() =>
  listOpen.value && highlight.value >= 0
    ? `${listId}-${highlight.value}`
    : undefined,
);

function commitTags(parts: string[]): void {
  const next = [...props.modelValue];
  for (const part of parts) {
    const v = part.trim();
    if (v && !next.includes(v)) next.push(v);
  }
  if (next.length === props.modelValue.length) return;
  emit('update:modelValue', next);
  emit('change');
}

function addTag(): void {
  const raw = tagInput.value;
  tagInput.value = '';
  commitTags(raw.split(TAG_SEPARATOR));
}

function onTagInput(event: Event): void {
  const el = event.target as HTMLInputElement;
  const parts = el.value.split(TAG_SEPARATOR);
  const rest = parts.pop() ?? '';
  if (parts.length > 0) {
    el.value = rest;
    commitTags(parts);
  }
  tagInput.value = rest;
  highlight.value = -1;
  dismissed.value = false;
}

function onFocus(): void {
  focused.value = true;
  dismissed.value = false;
  highlight.value = -1;
  if (!props.suggestionSource) return;
  try {
    knownTags.value = props.suggestionSource();
  } catch {
    knownTags.value = [];
  }
}

function onBlur(): void {
  focused.value = false;
  highlight.value = -1;
  addTag();
}

function selectSuggestion(tag: string): void {
  tagInput.value = '';
  if (inputEl.value) {
    inputEl.value.value = '';
    inputEl.value.focus();
  }
  highlight.value = -1;
  commitTags([tag]);
}

function onKeydown(event: KeyboardEvent): void {
  const count = suggestions.value.length;
  switch (event.key) {
    case 'ArrowDown':
      if (!listOpen.value) return;
      event.preventDefault();
      highlight.value = Math.min(highlight.value + 1, count - 1);
      return;
    case 'ArrowUp':
      if (!listOpen.value) return;
      event.preventDefault();
      highlight.value = Math.max(highlight.value - 1, -1);
      return;
    case 'Escape':
      if (!listOpen.value) return;
      event.preventDefault();
      dismissed.value = true;
      highlight.value = -1;
      return;
    case 'Enter': {
      event.preventDefault();
      const tag =
        listOpen.value && highlight.value >= 0
          ? suggestions.value[highlight.value]
          : undefined;
      if (tag !== undefined) selectSuggestion(tag);
      else addTag();
      return;
    }
  }
}

function removeTag(tag: string): void {
  emit(
    'update:modelValue',
    props.modelValue.filter((t) => t !== tag),
  );
  emit('change');
}
</script>

<template>
  <div class="tag-input-root">
    <div class="tags" data-test="edit-tags">
      <span v-for="tag in modelValue" :key="tag" class="tag-chip">
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
        ref="inputEl"
        :value="tagInput"
        type="text"
        class="tag-input"
        placeholder="Add tag, press space or comma"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck="false"
        enterkeyhint="done"
        role="combobox"
        aria-label="Tags"
        aria-autocomplete="list"
        :aria-expanded="listOpen"
        :aria-controls="listId"
        :aria-activedescendant="activeDescendant"
        data-test="edit-tag-input"
        @input="onTagInput"
        @keydown="onKeydown"
        @focus="onFocus"
        @blur="onBlur"
      />
    </div>
    <ul
      v-show="listOpen"
      :id="listId"
      class="tag-suggestions"
      role="listbox"
      data-test="edit-tag-suggestions"
      @pointerdown.prevent
      @mousedown.prevent
      @click.prevent
    >
      <li
        v-for="(tag, i) in suggestions"
        :id="`${listId}-${i}`"
        :key="tag"
        class="tag-suggestion"
        :class="{ highlighted: i === highlight }"
        role="option"
        :aria-selected="i === highlight"
        :data-test="`edit-tag-suggestion-${tag}`"
        @click="selectSuggestion(tag)"
      >
        {{ tag }}
      </li>
    </ul>
  </div>
</template>

<style scoped>
.tag-input-root {
  position: relative;
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
  width: 100%;
  min-width: 8rem;
  padding: 0.5rem 0.75rem;
  font-size: 1rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  outline: none;
  background: #ffffff;
  color: #111;
  font-family: inherit;
}

.tag-input:focus {
  outline: 2px solid #2563eb;
  outline-offset: -1px;
}

.tag-suggestions {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  z-index: 10;
  margin: 0.25rem 0 0;
  padding: 0.25rem 0;
  list-style: none;
  background: #ffffff;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.tag-suggestion {
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: #111;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tag-suggestion:hover {
  background: #f3f4f6;
}

.tag-suggestion.highlighted {
  background: #eff6ff;
}
</style>
