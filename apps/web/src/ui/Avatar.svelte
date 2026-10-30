<script>
  let { name = '', ...rest } = $props();
  const initials = $derived(name.split(/[\s@.]+/).filter(Boolean).slice(0, 2).map((w) => w[0].toUpperCase()).join('') || '?');
</script>

<span class="avatar" {...rest}>{initials}</span>

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
</style>
