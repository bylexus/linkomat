<script setup lang="ts">
</script>

<template>
  <footer class="app-footer" data-test="app-footer">
    <span class="app-footer-copy" data-test="app-footer-copyright">
      &copy; 2026
      <a
        href="https://alexi.ch/"
        target="_blank"
        rel="noopener noreferrer"
        class="app-footer-link"
        data-test="app-footer-alexi"
      >alexi.ch</a>
    </span>
    <RouterLink
      to="/about"
      class="app-footer-link"
      data-test="app-footer-about-link"
    >About</RouterLink>
  </footer>
</template>

<style scoped>
.app-footer {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  max-width: 720px;
  margin: 2rem auto 0;
  padding: 1rem;
  border-top: 1px solid #e5e7eb;
  color: #6b7280;
  font-size: 0.875rem;
}

.app-footer-link {
  color: #2563eb;
  text-decoration: underline;
}
</style>
