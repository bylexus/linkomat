import { ref, type Ref } from 'vue';

export type HomeStateRefs = {
  query: Ref<string>;
  numPages: Ref<number>;
  scrollY: Ref<number>;
  isPreserved: Ref<boolean>;
};

const refs: HomeStateRefs = {
  query: ref(''),
  numPages: ref(1),
  scrollY: ref(0),
  isPreserved: ref(false),
};

export function usePreservedHomeState(): HomeStateRefs {
  return refs;
}

export function clearPreservedHomeState(): void {
  refs.isPreserved.value = false;
  refs.query.value = '';
  refs.numPages.value = 1;
  refs.scrollY.value = 0;
}
