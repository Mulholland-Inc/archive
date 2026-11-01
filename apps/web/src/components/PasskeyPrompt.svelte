<script>
  import { KeyRound } from '@lucide/svelte';
  import Button from './Button.svelte';
  import Icon from './Icon.svelte';
  import { passkeyRegister, rememberPasskey } from '../lib/api.js';
  import { app } from '../lib/app.svelte.js';
  import { rise } from '../lib/motion.js';

  let busy = $state(false);
  let error = $state('');

  async function add() {
    busy = true;
    error = '';
    try {
      await passkeyRegister(app.user.id);
      app.askPasskey = false;
    } catch (e) {
      if (e.name !== 'NotAllowedError' && e.name !== 'AbortError') error = e.message;
    }
    busy = false;
  }

  function later() {
    rememberPasskey(app.user.id, 'no');
    app.askPasskey = false;
  }
</script>

<div class="prompt" role="dialog" aria-label="Add a passkey" in:rise>
  <span class="glyph"><Icon icon={KeyRound} size="md" /></span>
  <span class="text">
    <span class="title">Sign in faster on this device</span>
    <span class="sub">{error || 'Add a passkey, then sign in with your fingerprint, face or PIN.'}</span>
  </span>
  <Button size="sm" variant="ghost" onclick={later}>Not now</Button>
  <Button size="sm" variant="primary" loading={busy} onclick={add}>Add passkey</Button>
</div>

<style>
  .prompt {
    position: fixed;
    inset-inline: 0;
    bottom: var(--space-4);
    z-index: var(--z-dialog);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: max-content;
    max-width: calc(100% - 2 * var(--space-4));
    margin-inline: auto;
    padding: var(--space-2) var(--space-2) var(--space-2) var(--space-3);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--ring-strong);
  }

  .glyph {
    display: flex;
    flex: none;
    color: var(--color-text-secondary);
  }

  .text {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
    line-height: var(--leading-snug);
  }

  .title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .sub {
    overflow: hidden;
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
