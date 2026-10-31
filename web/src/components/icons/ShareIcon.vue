<script setup lang="ts">
withDefaults(defineProps<{ width?: string; height?: string }>(), {
  width: '1.5rem',
  height: '1.5rem',
});
</script>
<template>
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" :width="width" :height="height" fill="none"
    stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <circle cx="18" cy="5" r="2.75" />
    <circle cx="6" cy="12" r="2.75" />
    <circle cx="18" cy="19" r="2.75" />
    <path d="M8.4 13.4l7.2 4.2" />
    <path d="M15.6 6.4l-7.2 4.2" />
  </svg>
</template>
