<script>
  import { onMount } from 'svelte';
  import { check, mount } from '../lib/avatar.js';
  import { AVATARS } from '../lib/avatars.js';

  let { avatar, tint = 0, mood = 'idle', size = 'md' } = $props();

  // An agent writes its own avatar, so one the renderer refuses shows the fallback.
  const def = $derived.by(() => {
    const checked = check($state.snapshot(avatar));
    return checked.ok ? checked.value : AVATARS[0];
  });

  // The renderer takes colours, not tokens.
  const token = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const options = () => ({ def, tint: token(`--tint-${tint}`), ink: token('--color-avatar-ink'), mood });

  let host;
  let live;

  onMount(() => {
    live = mount(host, options());
    return () => live?.destroy();
  });

  $effect(() => {
    const next = options();
    live?.update(next);
  });
</script>

<span bind:this={host} class={['avatar', size]} aria-hidden="true"></span>

<style>
  .avatar {
    display: block;
    flex: none;
    width: var(--avatar-md);
    height: var(--avatar-md);
  }

  .xs { width: var(--avatar-xs); height: var(--avatar-xs); }
  .lg { width: var(--avatar-lg); height: var(--avatar-lg); }
  .xxl { width: var(--avatar-2xl); height: var(--avatar-2xl); }
</style>
