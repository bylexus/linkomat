import { ref, type Ref } from 'vue';
import type { Entry } from '../db/sqlite';

export type HomeStateRefs = {
  query: Ref<string>;
  entries: Ref<Entry[]>;
  cursor: Ref<{ updated_at: string; id: string } | null>;
  hasMore: Ref<boolean>;
  scrollY: Ref<number>;
  isPreserved: Ref<boolean>;
};

const refs: HomeStateRefs = {
  query: ref(''),
  entries: ref<Entry[]>([]),
  cursor: ref<{ updated_at: string; id: string } | null>(null),
  hasMore: ref(false),
  scrollY: ref(0),
  isPreserved: ref(false),
};

export function usePreservedHomeState(): HomeStateRefs {
  return refs;
}

export function clearPreservedHomeState(): void {
  refs.isPreserved.value = false;
  refs.query.value = '';
  refs.entries.value = [];
  refs.cursor.value = null;
  refs.hasMore.value = false;
  refs.scrollY.value = 0;
}
