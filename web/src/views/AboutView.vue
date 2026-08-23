<script setup lang="ts">
</script>

<template>
  <main class="about">
    <h1 class="about-title" data-test="about-title">About Linkomat</h1>
    <p class="about-body" data-test="about-body">
      Linkomat is a small, local-first bookmark and note store built for one person to use. All your entries live in the browser and sync in the background to the server. It is designed to be fast to open, fast to drop a link into, and works offline from the home screen of your phone.
    </p>
    <p class="about-copy" data-test="about-copy">
      &copy; 2026
      <a
        href="https://alexi.ch/"
        target="_blank"
        rel="noopener noreferrer"
        class="about-link"
        data-test="about-link"
      >alexi.ch</a>
    </p>
  </main>
</template>

<style scoped>
.about {
  max-width: 32rem;
  margin: 2rem auto;
  padding: 1rem;
}

.about-title {
  margin: 0 0 1rem 0;
  font-size: 1.5rem;
}

.about-body {
  color: #374151;
  line-height: 1.6;
  margin: 0 0 1rem 0;
}

.about-copy {
  color: #6b7280;
  font-size: 0.875rem;
  margin: 0;
}

.about-link {
  color: #2563eb;
  text-decoration: underline;
}
</style>
