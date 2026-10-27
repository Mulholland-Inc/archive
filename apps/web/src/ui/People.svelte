<script>
  let { people, live = true } = $props();
</script>

<div class="people" class:live>
  {#each people as p (p.key)}
    <span class="person" class:pulse={p.pulse} data-name={p.name} aria-label={p.name} role="img">{(p.name || '?')[0].toUpperCase()}</span>
  {/each}
</div>

<style>
  .people { display: flex; align-items: center; padding: var(--space-1); opacity: 0.5; transition: opacity var(--dur) var(--ease); }
  .live { opacity: 1; }
  .person {
    position: relative;
    display: grid;
    place-items: center;
    width: var(--size-sm);
    height: var(--size-sm);
    margin: calc(-1 * var(--space-1));
    border-radius: 50%;
    background: var(--raised);
    box-shadow: 0 0 0 var(--stroke-bold) var(--ring);
    color: var(--text);
    font-size: var(--text-sm);
    font-weight: var(--weight-bold);
    cursor: default;
  }
  .person:hover { z-index: 1; }
  .person::after {
    content: attr(data-name);
    position: absolute;
    top: calc(100% + var(--space-2));
    right: 0;
    padding: var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--raised);
    box-shadow: var(--shadow);
    color: var(--text);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    white-space: nowrap;
    opacity: 0;
    pointer-events: none;
    transition: opacity var(--dur) var(--ease);
  }
  .person:hover::after { opacity: 1; }
  .pulse { animation: pulse 1s infinite alternate; }
  @keyframes pulse { to { opacity: 0.5; } }
</style>
