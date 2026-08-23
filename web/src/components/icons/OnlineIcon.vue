<script setup lang="ts">
withDefaults(defineProps<{ width?: string; height?: string }>(), {
  width: '1.5rem',
  height: '1.5rem',
});
</script>
<template>
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" :width="width" :height="height" fill="none"
    stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M5 12.55a11 11 0 0 1 14 0" />
    <path d="M8.5 16.05a6 6 0 0 1 7 0" />
    <path d="M12 19.5h.01" />
  </svg>
</template>