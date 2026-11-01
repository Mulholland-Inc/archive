<script>
  import { avatarShape } from '../lib/avatarShape.js';

  let { name = '', seed, src, size = 'md' } = $props();
  const small = $derived(size === 'xs' || size === 'sm');
  const shape = $derived(avatarShape(seed || name, { small }));
  const stroke = $derived({ xs: 2.6, sm: 2.3, md: 1.9, ml: 1.8, lg: 1.7, xl: 1.4 }[size] ?? 1.9);
</script>

<span class={['avatar', size]} role="img" aria-label={name} title={name}>
  {#if src}
    <img {src} alt="" />
  {:else}
    <svg viewBox="0 0 64 64" aria-hidden="true">
      <circle class="disc" cx="32" cy="40" r="24" />
      {#each shape.solids as s, i (i)}
        {#each s.faces as f, j (j)}
          <path d={f.d} fill={f.fill} fill-rule={f.evenodd ? 'evenodd' : undefined} />
        {/each}
        {#each s.edges as d, j (j)}
          <path {d} class="ink" stroke-width={stroke * 0.8} />
        {/each}
        {#each s.outline as d, j (j)}
          <path {d} class="ink" stroke-width={stroke} />
        {/each}
      {/each}
    </svg>
  {/if}
</span>

<style>
  .avatar {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: var(--avatar-md);
    height: var(--avatar-md);
    user-select: none;
  }

  svg {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible;
  }

  img {
    width: 100%;
    height: 100%;
    border-radius: var(--radius-full);
    object-fit: cover;
  }

  .disc {
    fill: var(--color-fill-strong);
  }

  .ink {
    fill: none;
    stroke: var(--color-avatar-ink);
    stroke-linejoin: round;
    stroke-linecap: round;
  }

  .xs {
    width: var(--avatar-xs);
    height: var(--avatar-xs);
  }

  .sm {
    width: var(--avatar-sm);
    height: var(--avatar-sm);
  }

  .ml {
    width: var(--avatar-ml);
    height: var(--avatar-ml);
  }

  .lg {
    width: var(--avatar-lg);
    height: var(--avatar-lg);
  }

  .xl {
    width: var(--avatar-xl);
    height: var(--avatar-xl);
  }
</style>
