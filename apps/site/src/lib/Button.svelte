<script>
  let { href, ghost = false, small = false, children } = $props();
</script>

<a class={['btn', ghost && 'btn-ghost', small && 'btn-sm']} {href}
  ><i><svg viewBox="0 0 24 24"><path d="M5 12h14M13 6l6 6-6 6" /></svg></i><span>{@render children()}</span></a
>
