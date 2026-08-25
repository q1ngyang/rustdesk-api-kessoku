<template>
  <main class="action-shell">
    <div class="action-shell__glow" aria-hidden="true"></div>
    <section class="action-panel">
      <KessokuBrand/>
      <div class="action-panel__card"><slot/></div>
    </section>
  </main>
</template>

<script setup>
import KessokuBrand from '@/components/brand/KessokuBrand.vue'
</script>

<style scoped lang="scss">
.action-shell { position: relative; display: grid; min-height: 100dvh; overflow: hidden; place-items: center; padding: max(24px, env(safe-area-inset-top)) 16px max(24px, env(safe-area-inset-bottom)); background: var(--app-bg); }
.action-shell::before { position: absolute; inset: 0; background-image: linear-gradient(rgba(98, 113, 143, .04) 1px, transparent 1px),linear-gradient(90deg, rgba(98, 113, 143, .04) 1px, transparent 1px); background-size: 32px 32px; content: ''; }
.action-shell__glow { position: absolute; top: 50%; left: 50%; width: min(80vw, 720px); aspect-ratio: 1; border-radius: 50%; background: radial-gradient(circle, rgba(103, 119, 239, .14), transparent 67%); transform: translate(-50%, -50%); }
.action-panel { position: relative; z-index: 1; width: min(500px, 100%); }
.action-panel__card { margin-top: 22px; padding: clamp(24px, 6vw, 38px); border: 1px solid var(--border-subtle); border-radius: 24px; background: color-mix(in srgb, var(--surface-1) 95%, transparent); box-shadow: var(--shadow-lg); backdrop-filter: blur(18px); }
@media (max-width: 520px) { .action-panel__card { margin-top: 16px; padding: 22px 18px; border-radius: 20px; } }
</style>
