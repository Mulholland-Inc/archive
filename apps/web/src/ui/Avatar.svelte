<script>
  let { name = '', peer, pulse = false, ...rest } = $props();
  const initials = $derived(name.split(/[\s@.]+/).filter(Boolean).slice(0, peer === undefined ? 2 : 1).map((w) => w[0].toUpperCase()).join('') || '?');
</script>

<span class={['avatar', peer !== undefined && `tinted peer-${peer}`, pulse && 'pulse']} {...rest}>{initials}</span>

<style>
  .avatar {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--size-avatar);
    height: var(--size-avatar);
    border-radius: var(--radius-full);
    background: var(--fill-strong);
    color: var(--fg);
    font-size: var(--text-meta);
    font-weight: var(--weight-semibold);
    line-height: var(--leading-none);
  }
  .tinted { background: var(--peer); color: var(--on-peer); }
  .pulse { animation: pulse var(--duration-slow) infinite alternate; }
  @keyframes pulse { to { opacity: var(--opacity-dim); } }
</style>
