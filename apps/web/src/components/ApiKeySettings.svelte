<script>
  import { Eye, EyeOff, Copy, RefreshCw, Sparkles } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import OwnerNotice from './OwnerNotice.svelte';
  import Art from './Art.svelte';
  import IconButton from './IconButton.svelte';
  import Button from './Button.svelte';
  import { app, ws, vaultKey, rotateKey, isVaultOwner, tokens } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';
  import { apiPrompt } from '../lib/apiprompt.js';

  let { scope = 'user', onback, backNarrow = true } = $props();

  let shown = $state(false);
  const vault = $derived(scope === 'vault');
  const key = $derived(vault ? vaultKey() : app.apiKey);
  const locked = $derived(vault && !isVaultOwner());
  const secret = $derived(key?.secret ?? '');
  // A real token's secret is only known right after it is made; before that the key shows as dots.
  const hidden = $derived(vault && !secret);
  const masked = $derived(secret ? `${secret.slice(0, secret.lastIndexOf('_') + 1)}${'•'.repeat(12)}${secret.slice(-4)}` : key ? `arc_${'•'.repeat(16)}` : 'No key yet');

  tokens.use(() => vault && app.vault);

  async function copy() {
    await navigator.clipboard?.writeText(secret).catch(() => {});
  }

  async function copyPrompt() {
    await navigator.clipboard?.writeText(apiPrompt({ key: secret, vault: vault ? ws() : null })).catch(() => {});
  }

  async function rotate() {
    if (!key || (await confirmDialog({ title: 'Rotate this key?', body: 'The old key stops working right away.', confirm: 'Rotate' }))) {
      await rotateKey(scope);
      shown = vault;
    }
  }
</script>

<SettingsPane {onback} {backNarrow}>
  <div class="page">
    <div class="art"><Art name={vault ? 'security' : 'tokens'} /></div>
    <h2>{vault ? 'Vault API key' : 'Your API key'}</h2>
    <p class="sub">{vault ? `Only for ${ws().name}, with your access. Archive shows it once, when it is made; rotate it to see a new one.` : 'Works across every vault you’re in, with your access. Treat it like a password.'}</p>
    {#if locked}<div class="notice"><OwnerNotice what="the vault API key" ask="if you need it" /></div>{/if}
    <div class="key">
      <span class="value" aria-label={shown ? 'API key' : 'API key, hidden'}>{shown && !locked && secret ? secret : masked}</span>
      <IconButton icon={shown ? EyeOff : Eye} label={shown ? 'Hide key' : 'Show key'} size="sm" disabled={locked || hidden} onclick={() => (shown = !shown)} />
      <IconButton icon={RefreshCw} label={key ? 'Rotate key' : 'Make a key'} size="sm" disabled={locked} onclick={rotate} />
      <IconButton icon={Copy} label="Copy key" size="sm" disabled={locked || hidden} onclick={copy} />
    </div>
    <div class="setup">
      <Button variant="primary" icon={Sparkles} disabled={locked || hidden} onclick={copyPrompt}>Copy setup prompt</Button>
      <p>Paste it into ChatGPT, Claude or Cursor. It includes your key and the full API reference, so the assistant can start right away.</p>
    </div>
  </div>
</SettingsPane>

<style>
  .page {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-height: 100%;
    padding-block: var(--space-6);
    text-align: center;
  }

  .art {
    display: flex;
    height: var(--settings-art);
    margin-block-end: var(--space-6);
  }

  .art :global(img) {
    height: 100%;
  }

  h2 {
    font-family: var(--font-serif);
    font-size: var(--text-2xl);
    font-weight: var(--weight-serif);
    line-height: var(--leading-tight);
    letter-spacing: var(--tracking-serif);
  }

  .sub {
    max-width: 30rem;
    margin-block: var(--space-2) var(--space-6);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
  }

  .notice {
    width: 100%;
    max-width: 30rem;
    text-align: start;
  }

  .key {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    width: 100%;
    max-width: 30rem;
    min-height: var(--control-xl);
    padding: var(--space-1) var(--space-1) var(--space-1) var(--space-4);
    border-radius: var(--radius-lg);
    background: var(--color-fill-subtle);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .value {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-align: start;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-md);
    font-variant-numeric: tabular-nums;
    letter-spacing: var(--tracking-snug);
  }

  .setup {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    max-width: 24rem;
    margin-block-start: var(--space-5);
  }

  .setup p {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  @container (max-width: 32rem) {

    .art {
      height: calc(var(--settings-art) * 0.7);
    }

    h2 {
      font-size: var(--text-xl);
    }
  }
</style>
