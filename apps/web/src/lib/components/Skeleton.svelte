<script lang="ts">
  let { rows = 6 }: { rows?: number } = $props();
</script>

<div class="skeleton" aria-busy="true" aria-label="Loading">
  {#each { length: rows } as _, i (i)}
    <span class="line" style:--i={i}></span>
  {/each}
</div>

<style>
  .skeleton {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .line {
    height: var(--space-6);
    border-radius: var(--radius-md);
    background: var(--tint-2);
    animation: pulse var(--duration-slower) var(--ease-in-out) infinite alternate;
    animation-delay: calc(var(--i) * var(--stagger) * 3);
  }
  .line:nth-child(3n + 2) {
    width: 82%;
  }
  .line:nth-child(3n) {
    width: 64%;
  }
  @keyframes pulse {
    to {
      opacity: var(--opacity-soft);
    }
  }
</style>
