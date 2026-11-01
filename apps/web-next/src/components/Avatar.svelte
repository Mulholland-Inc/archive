<script>
  import { avatarShape } from '../lib/avatarShape.js';

  let { name = '', seed, src, size = 'md' } = $props();
  const small = $derived(size === 'xs' || size === 'sm');
  const shape = $derived(avatarShape(seed || name, { small }));
  const stroke = $derived({ xs: 2.6, sm: 2.2, md: 1.8, ml: 1.7, lg: 1.6, xl: 1.3 }[size] ?? 1.8);
</script>

<span class={['avatar', size, `t${shape.tile}`]} role="img" aria-label={name} title={name}>
  {#if src}
    <img {src} alt="" />
  {:else}
    <svg viewBox="0 0 64 64" aria-hidden="true">
      {#each shape.solids as s, i (i)}
        {#each s.faces as f, j (j)}
          <path d={f.d} fill={f.hole ? undefined : f.fill} fill-rule={f.evenodd ? 'evenodd' : undefined} class={{ hole: f.hole }} />
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
    --tile: var(--color-avatar-1);
    display: inline-grid;
    place-items: center;
    flex: none;
    width: var(--avatar-md);
    height: var(--avatar-md);
    overflow: hidden;
    border-radius: var(--radius-sm);
    background: var(--tile);
    user-select: none;
  }

  img,
  svg {
    display: block;
    width: 100%;
    height: 100%;
  }

  img {
    object-fit: cover;
  }

  .ink {
    fill: none;
    stroke: var(--color-avatar-ink);
    stroke-linejoin: round;
    stroke-linecap: round;
  }

  .hole {
    fill: var(--tile);
  }

  .xs {
    width: var(--avatar-xs);
    height: var(--avatar-xs);
    border-radius: var(--radius-xs);
  }

  .sm {
    width: var(--avatar-sm);
    height: var(--avatar-sm);
    border-radius: var(--radius-xs);
  }

  .ml {
    width: var(--avatar-ml);
    height: var(--avatar-ml);
    border-radius: var(--radius-sm);
  }

  .lg {
    width: var(--avatar-lg);
    height: var(--avatar-lg);
    border-radius: var(--radius-md);
  }

  .xl {
    width: var(--avatar-xl);
    height: var(--avatar-xl);
    border-radius: var(--radius-xl);
  }

  .t2 { --tile: var(--color-avatar-2); }
  .t3 { --tile: var(--color-avatar-3); }
  .t4 { --tile: var(--color-avatar-4); }
  .t5 { --tile: var(--color-avatar-5); }
  .t6 { --tile: var(--color-avatar-6); }
</style>
