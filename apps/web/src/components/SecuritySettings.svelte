<script>
  import { Copy } from '@lucide/svelte';
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import CodeInput from './CodeInput.svelte';
  import Heading from './Heading.svelte';
  import IconButton from './IconButton.svelte';
  import { api, said } from '../lib/api.js';

  let on = $state(null);
  let link = $state(null);
  let unlinking = $state(false);
  let code = $state('');
  let error = $state('');
  let busy = $state(false);

  let qr = $state(null);
  const dark = $derived(qr && qr.data.flatMap((row, y) => row.flatMap((cell, x) => (cell ? [`M${x} ${y}h1v1h-1z`] : []))).join(''));

  const plate = $derived(qr && Math.round(qr.size * 0.24));
  const corner = $derived(qr && (qr.size - plate) / 2);

  api('/auth/totp')
    .then((out) => (on = out.on))
    .catch((e) => (error = said(e)));

  async function act(work) {
    error = '';
    busy = true;
    try {
      await work();
    } catch (e) {
      error = said(e);
    }
    code = '';
    busy = false;
  }

  const start = () =>
    act(async () => {
      const [got, { encode }] = await Promise.all([api('/auth/totp/setup', { method: 'POST' }), import('uqr')]);
      qr = encode(got.uri, { border: 4, ecc: 'H' });
      link = got;
    });

  const post = async (path) => (on = (await api(path, { method: 'POST', body: { code } })).on);

  const confirm = () =>
    act(async () => {
      await post('/auth/totp/enable');
      link = null;
    });

  const unlink = () =>
    act(async () => {
      await post('/auth/totp/disable');
      unlinking = false;
    });

  function cancel() {
    link = null;
    unlinking = false;
    code = error = '';
  }
</script>

{#snippet failure()}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
{/snippet}

<div class="page">
  {#if link}
    <div class="card">
      <svg class="qr" viewBox="0 0 {qr.size} {qr.size}" shape-rendering="crispEdges" role="img" aria-label="QR code"><path d={dark} /><image href="{import.meta.env.BASE_URL}art/shield.webp" x={corner} y={corner} width={plate} height={plate} /></svg>
      <Heading size="xl" wrap="normal">Scan, then enter your code</Heading>
      <div class="key">
        <span class="value">{link.key}</span>
        <IconButton icon={Copy} label="Copy key" size="sm" onclick={() => navigator.clipboard?.writeText(link.key).catch(() => {})} />
      </div>
      <CodeInput bind:value={code} {busy} invalid={error && !code} done={confirm} />
      {@render failure()}
      <Button variant="ghost" onclick={cancel}>Cancel</Button>
    </div>
  {:else if unlinking}
    <div class="card">
      <Art name="linked" />
      <Heading size="xl" wrap="normal">Enter your code to unlink</Heading>
      <CodeInput bind:value={code} {busy} invalid={error && !code} done={unlink} />
      {@render failure()}
      <Button variant="ghost" onclick={cancel}>Cancel</Button>
    </div>
  {:else if on !== null}
    <div class="card">
      <Art name={on ? 'linked' : 'unlinked'} />
      <Heading size="xl" wrap="normal">Authenticator app</Heading>
      {@render failure()}
      {#if on}
        <Button onclick={() => (unlinking = true)}>Unlink</Button>
      {:else}
        <Button variant="primary" loading={busy} onclick={start}>Link</Button>
      {/if}
    </div>
  {:else}
    {@render failure()}
  {/if}
</div>

<style>
  .page {
    display: grid;
    place-items: center;
    min-height: 100%;
    padding-block: var(--space-4);
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    width: 100%;
    max-width: var(--measure-xs);
    text-align: center;
  }

  .qr {
    width: 100%;
    max-width: var(--settings-qr);
    border-radius: var(--radius-lg);
    background: #fff;
    fill: #000;
  }

  .key {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    width: 100%;
    min-height: var(--control-xl);
    padding: var(--space-1) var(--space-1) var(--space-1) var(--space-4);
    border-radius: var(--radius-lg);
    background: var(--color-fill-subtle);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
  }

  .value {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    font-size: var(--text-sm);
    text-align: start;
  }

  .error {
    color: var(--color-danger);
    font-size: var(--text-sm);
  }
</style>
