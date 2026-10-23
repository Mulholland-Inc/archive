<script lang="ts">
  let { token }: { token: string } = $props();
  let el: HTMLElement;
  let value = $state('');
  $effect(() => {
    token;
    const read = () => (value = getComputedStyle(el).getPropertyValue(token).trim());
    read();
    const mo = new MutationObserver(read);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    return () => mo.disconnect();
  });
</script>

<div class="swatch" bind:this={el}>
  <span class="chip" style:background={`var(${token})`}></span>
  <span class="meta">
    <span class="token">{token}</span>
    <span class="value">{value}</span>
  </span>
</div>

<style>
  .swatch {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }
  .chip {
    flex: none;
    width: var(--space-10);
    height: var(--space-10);
    border-radius: var(--radius-md);
    box-shadow: inset 0 0 0 var(--border-width) var(--tint-2);
  }
  .meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
    line-height: var(--leading-snug);
  }
  .token {
    font-size: var(--text-sm);
  }
  .value {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--muted-foreground);
  }
</style>
