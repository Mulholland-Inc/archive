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
    border-radius: 50%;
    background: var(--cream);
    color: var(--cream-ink);
    font-size: var(--text-xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0;
  }
  .tinted { background: var(--peer); color: var(--on-peer); }
  .pulse { animation: pulse 1s infinite alternate; }
  @keyframes pulse { to { opacity: 0.35; } }
</style>
