<template>
  <main class="embed-host">
    <iframe
      class="embed-frame"
      :src="embedSrc"
      title="GPT Image Tool"
      allow="clipboard-read; clipboard-write"
    ></iframe>
  </main>
</template>

<script setup lang="ts">
const base = import.meta.env.BASE_URL.replace(/\/$/, '')
const embedSrc = `${base}/image-tool/embed`
</script>

<style scoped>
.embed-host {
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #ffffff;
}

.embed-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}
</style>
