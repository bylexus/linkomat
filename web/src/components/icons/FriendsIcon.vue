<script setup lang="ts">
withDefaults(defineProps<{ width?: string; height?: string }>(), {
  width: '1.5rem',
  height: '1.5rem',
});
</script>
<template>
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" :width="width" :height="height" fill="none"
    stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle cx="9" cy="8" r="3.5" />
    <path d="M2.5 20.5a6.5 6.5 0 0 1 13 0" />
    <path d="M15.5 4.75a3.25 3.25 0 0 1 0 6.5" />
    <path d="M18 14.75a6.5 6.5 0 0 1 3.5 5.75" />
  </svg>
</template>
