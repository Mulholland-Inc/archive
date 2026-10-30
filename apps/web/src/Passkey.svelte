<script>
  import { passkeyRegister, rememberPasskey } from './api.js';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';

  let { user, ondone } = $props();
  let busy = $state(false);

  async function add() {
    busy = true;
    try {
      await passkeyRegister(user);
      ondone();
    } catch {}
    busy = false;
  }

  function later() {
    rememberPasskey(user, 'no');
    ondone();
  }
</script>

<div class="prompt" role="dialog" aria-label="Add a passkey">
  <Icon name="key-round" />
  <span>Add a passkey</span>
  <Button ghost small onclick={later}>Not now</Button>
  <Button small disabled={busy} onclick={add}>Add passkey</Button>
</div>

<style>
  .prompt {
    display: flex;
    flex: none;
    align-items: center;
    align-self: center;
    gap: var(--space-2);
    max-width: 100%;
    margin-bottom: var(--frame);
    padding: var(--space-2);
    border-radius: var(--radius-full);
    background: var(--surface-raised);
    color: var(--fg-soft);
    font-size: var(--text-ui);
    line-height: var(--leading-snug);
    animation: rise var(--duration-fast) var(--ease-out) both;
  }
  span { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  @keyframes rise { from { opacity: 0; transform: translateY(var(--rise)); } }
  @media (prefers-reduced-motion: reduce) { .prompt { animation: none; } }
</style>
