import { ref } from 'vue';

const preserved = ref('');

export function usePreservedHomeSearch() {
  return preserved;
}