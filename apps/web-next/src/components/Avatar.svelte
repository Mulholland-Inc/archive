<script>
  import { initials, toneFor } from '../lib/data.js';

  let { name = '', src, size = 'md', square = false, tone, ring = false, status } = $props();
  const t = $derived(tone ?? toneFor(name));
</script>

<span class={['avatar', size, `t${t}`, { square, ring }]} title={name}>
  {#if src}
    <img {src} alt={name} />
  {:else}
    <span aria-hidden="true">{size === 'xs' || size === 'sm' ? initials(name).slice(0, 1) : initials(name)}</span>
    <span class="visually-hidden">{name}</span>
  {/if}
  {#if status}<span class={['status', status]}></span>{/if}
</span>

<style>
  .avatar {
    position: relative;
    display: inline-grid;
    place-items: center;
    flex: none;
    width: var(--avatar-md);
    height: var(--avatar-md);
    border-radius: var(--radius-full);
    color: var(--color-avatar-text);
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    letter-spacing: var(--tracking-wide);
    line-height: var(--leading-none);
    user-select: none;
  }

  img {
    width: 100%;
    height: 100%;
    border-radius: inherit;
    object-fit: cover;
  }

  .xs {
    width: var(--avatar-xs);
    height: var(--avatar-xs);
    font-size: var(--text-2xs);
    letter-spacing: var(--tracking-normal);
  }

  .sm {
    width: var(--avatar-sm);
    height: var(--avatar-sm);
    font-size: var(--text-2xs);
  }

  .lg {
    width: var(--avatar-lg);
    height: var(--avatar-lg);
    font-size: var(--text-md);
  }

  .xl {
    width: var(--avatar-xl);
    height: var(--avatar-xl);
    font-size: var(--text-xl);
  }

  .square {
    border-radius: var(--radius-md);
  }

  .square.xs,
  .square.sm {
    border-radius: var(--radius-sm);
  }

  .square.xl {
    border-radius: var(--radius-xl);
  }

  .ring {
    box-shadow: 0 0 0 var(--focus-width) var(--color-canvas);
  }

  .t1 { background: var(--color-avatar-1); }
  .t2 { background: var(--color-avatar-2); }
  .t3 { background: var(--color-avatar-3); }
  .t4 { background: var(--color-avatar-4); }
  .t5 { background: var(--color-avatar-5); }
  .t6 { background: var(--color-avatar-6); }

  .status {
    position: absolute;
    inset-inline-end: calc(var(--space-0-5) * -1);
    inset-block-end: calc(var(--space-0-5) * -1);
    width: var(--space-2);
    height: var(--space-2);
    border-radius: var(--radius-full);
    box-shadow: 0 0 0 var(--focus-width) var(--avatar-cutout, var(--color-canvas));
    background: var(--color-text-tertiary);
  }

  .online {
    background: var(--color-success);
  }
</style>
