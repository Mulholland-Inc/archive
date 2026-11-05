<script>
  import { onMount } from 'svelte';
  import { validateAvatar } from 'open-bot-avatar';
  import { mountAvatar } from 'open-bot-avatar/dom';
  import { AVATARS } from '../lib/avatars.js';

  let { avatar, tint = 0, mood = 'idle', size = 'md' } = $props();

  const fallback = AVATARS.find((a) => a.id === 'chat');

  // An agent writes its own avatar, so one the renderer refuses shows the fallback.
  const definition = $derived.by(() => {
    const checked = validateAvatar($state.snapshot(avatar));
    return checked.ok ? checked.value : fallback;
  });

  // The renderer takes a colour, not a token, and the eyes stay readable on any tint.
  const color = $derived(getComputedStyle(document.documentElement).getPropertyValue(`--tint-${tint}`).trim());
  const eyeColor = $derived.by(() => {
    const n = parseInt(color.slice(1), 16);
    return 0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255) < 90 ? '#ffffff' : '#151515';
  });

  let host;
  let live;
  const options = () => ({ definition, color, eyeColor, mood, followPointer: true, label: '' });

  onMount(() => {
    live = mountAvatar(host, options());
    return () => live.destroy();
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
