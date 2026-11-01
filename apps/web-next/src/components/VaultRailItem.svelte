<script>
  import Tooltip from './Tooltip.svelte';

  let { label, active = false, count = 0, add = false, onclick, children } = $props();
</script>

<div class={['item', { active, add }]}>
  <span class="pill" aria-hidden="true"></span>
  <Tooltip text={label} placement="right" size="md" instant>
    <button type="button" class="tile" aria-label={count ? `${label}, ${count} running` : label} aria-current={active ? 'true' : undefined} {onclick}>
      {@render children()}
      {#if count}<span class="badge" aria-hidden="true">{count}</span>{/if}
    </button>
  </Tooltip>
</div>

<style>
  .item {
    position: relative;
    display: flex;
    justify-content: center;
    width: 100%;
  }

  .pill {
    position: absolute;
    inset-inline-start: 0;
    top: 50%;
    width: var(--vault-pill);
    height: var(--space-2);
    border-radius: 0 var(--radius-full) var(--radius-full) 0;
    background: var(--color-rail-pill);
    opacity: 0;
    transform: translate(-100%, -50%);
    transition:
      height var(--duration-normal) var(--ease-out),
      opacity var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-out);
  }

  .item:hover .pill {
    height: var(--space-5);
    opacity: 1;
    transform: translate(0, -50%);
  }

  .active .pill,
  .active:hover .pill {
    height: var(--space-10);
    opacity: 1;
    transform: translate(0, -50%);
  }

  .add .pill {
    display: none;
  }

  .tile {
    position: relative;
    display: grid;
    place-items: center;
    width: var(--vault-tile);
    height: var(--vault-tile);
    border-radius: var(--radius-full);
    background: var(--color-rail-tile);
    color: var(--color-rail-icon);
    transition:
      border-radius var(--duration-normal) var(--ease-out),
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      transform var(--duration-instant) var(--ease-out);
  }

  .tile:hover,
  .active .tile {
    border-radius: var(--radius-lg);
    background: var(--color-rail-hover);
  }

  .tile:active {
    transform: scale(var(--scale-press));
  }

  .add .tile {
    color: var(--color-rail-accent);
  }

  .add .tile:hover {
    background: var(--color-rail-accent);
    color: var(--color-rail-on-accent);
  }

  .badge {
    position: absolute;
    inset-block-end: calc(var(--space-0-5) * -1);
    inset-inline-end: calc(var(--space-0-5) * -1);
    display: grid;
    place-items: center;
    min-width: var(--vault-badge);
    height: var(--vault-badge);
    padding-inline: var(--space-1);
    border-radius: var(--radius-full);
    background: var(--color-rail-badge);
    color: var(--color-on-rail-badge);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    line-height: var(--leading-none);
    box-shadow: 0 0 0 var(--space-1) var(--color-rail);
  }

  @media (prefers-reduced-motion: reduce) {
    .pill,
    .tile {
      transition: none;
    }
  }
</style>
