<script>
  import { Eye, EyeOff, Copy, RefreshCw, Sparkles } from '@lucide/svelte';
  import SettingsPane from './SettingsPane.svelte';
  import Art from './Art.svelte';
  import Heading from './Heading.svelte';
  import IconButton from './IconButton.svelte';
  import Button from './Button.svelte';
  import { app, ws, archiveKey, rotateKey, tokens, revokeApp } from '../lib/app.svelte.js';
  import { confirmDialog } from '../lib/dialogs.svelte.js';
  import { apiPrompt } from '../lib/apiprompt.js';

  let { personal = false, onback, backNarrow = true } = $props();

  let shown = $state(false);
  const key = $derived(personal ? app.apiKey : archiveKey());
  const secret = $derived(key?.secret ?? '');
  const masked = $derived(secret ? `${secret.slice(0, secret.lastIndexOf('_') + 1)}${'•'.repeat(12)}${secret.slice(-4)}` : key ? `arc_${'•'.repeat(16)}` : 'No key yet');

  tokens.use(() => app.archive);

  async function current() {
    if (!secret) await rotateKey(personal);
    return (personal ? app.apiKey : archiveKey())?.secret ?? '';
  }

  async function copy() {
    await navigator.clipboard?.writeText(await current()).catch(() => {});
  }

  async function copyPrompt() {
    await navigator.clipboard?.writeText(apiPrompt({ key: await current(), archive: personal ? null : ws(), archives: app.archives })).catch(() => {});
  }

  const since = (t) => `Connected ${new Date(t.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}`;

  async function disconnect(t) {
    if (await confirmDialog({ title: `Disconnect ${t.name}?`, body: `It loses access to ${t.workspace_name} right away.`, confirm: 'Disconnect' })) await revokeApp(t.id);
  }

  async function rotate() {
    if (!key || (await confirmDialog({ title: 'Rotate this key?', body: 'The old key stops working right away.', confirm: 'Rotate' }))) {
      await rotateKey(personal);
      shown = true;
    }
  }
</script>

<SettingsPane {onback} {backNarrow}>
  <div class="page">
    <div class="art"><Art name={personal ? 'tokens' : 'security'} /></div>
    <Heading size="xl" wrap="normal" step>{personal ? 'Your API key' : 'Archive API key'}</Heading>
    <p class="sub">{personal ? 'Works in every archive you’re in' : `Only for ${ws().name}`}, with your access. Copy it whenever you need it.</p>
    <div class="key">
      <span class="value" aria-label={shown ? 'API key' : 'API key, hidden'}>{shown && secret ? secret : masked}</span>
      <IconButton icon={shown ? EyeOff : Eye} label={shown ? 'Hide key' : 'Show key'} size="sm" disabled={!secret} onclick={() => (shown = !shown)} />
      <IconButton icon={RefreshCw} label={key ? 'Rotate key' : 'Make a key'} size="sm" onclick={rotate} />
      <IconButton icon={Copy} label="Copy key" size="sm" onclick={copy} />
    </div>
    <div class="setup">
      <Button variant="primary" icon={Sparkles} onclick={copyPrompt}>Copy setup prompt</Button>
    </div>
    {#if personal && app.apps.length}
      <div class="apps">
        <Heading level={3} size="sm" serif={false}>Connected apps</Heading>
        {#each app.apps as t (t.id)}
          <div class="app">
            <span class="app-name">{t.name}<small>{since(t)} to {t.workspace_name}, as you</small></span>
            <Button size="sm" variant="ghost" onclick={() => disconnect(t)}>Disconnect</Button>
          </div>
        {/each}
      </div>
    {/if}
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

  .sub {
    max-width: var(--measure-sm);
    margin-block: var(--space-2) var(--space-6);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
  }

  .key {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    width: 100%;
    max-width: var(--measure-sm);
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
    max-width: var(--measure-xs);
    margin-block-start: var(--space-5);
  }

  .apps {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    width: 100%;
    max-width: var(--measure-sm);
    margin-block-start: var(--space-8);
    text-align: start;
  }

  .app {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: var(--control-xl);
    padding-block: var(--space-1);
  }

  .app + .app {
    box-shadow: inset 0 var(--border-width) 0 var(--color-border-subtle);
  }

  .app-name {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .app-name small {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-weight: var(--weight-regular);
  }

  @container (max-width: 32rem) {
    .art {
      height: var(--settings-art-sm);
    }
  }
</style>
