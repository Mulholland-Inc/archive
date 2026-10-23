<script lang="ts">
  let { rows = 6 }: { rows?: number } = $props();
</script>

<div class="skeleton" aria-busy="true" aria-label="Loading">
  {#each { length: rows } as _, i (i)}
    <span class="line"></span>
  {/each}
</div>

<style>
  .skeleton {
    animation: appear 0s 400ms both;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .line {
    height: var(--space-6);
    border-radius: var(--radius-md);
    background: var(--tint-2);
  }
  .line:nth-child(3n + 2) {
    width: 82%;
  }
  .line:nth-child(3n) {
    width: 64%;
  }
  @keyframes appear {
    from {
      visibility: hidden;
    }
  }
</style>
