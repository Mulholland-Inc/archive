<script>
  import { Check } from '@lucide/svelte';
  import Art from '../components/Art.svelte';
  import ArchiveIcon from '../components/ArchiveIcon.svelte';
  import Button from '../components/Button.svelte';
  import Heading from '../components/Heading.svelte';
  import Icon from '../components/Icon.svelte';
  import Loading from '../components/Loading.svelte';
  import SignIn from '../components/SignIn.svelte';
  import Text from '../components/Text.svelte';
  import { api, signOut, workspace } from '../lib/api.js';
  import { lookOf, TYPES } from '../lib/looks.js';
  import { fade } from '../lib/motion.js';

  // An MCP client, like Claude or ChatGPT, asking to be let into one of your archives: mcp.<domain>/oauth/authorize
  // checked its request and sent you here, sealed, as ?request= (src/mcp.go). You sign in as you do to the app,
  // pick an archive and allow it; the API answers with where to send the browser, back to the client. The
  // client is named as it registered itself (client_name), or "an app" if it gave no name.

  const query = new URLSearchParams(location.search);
  const request = query.get('request') ?? '';
  const framed = window.top !== window.self;

  let state = $state(framed ? 'error' : query.get('error') ? 'error' : 'loading');
  let error = $state(framed ? 'Archive won’t ask for access inside another page. Open this link in its own tab.' : (query.get('error') ?? ''));
  let client = $state({ name: '' });
  let user = $state(null);
  let archives = $state([]);
  let picked = $state('');
  let busy = $state(false);

  const chosen = $derived(archives.find((w) => w.id === picked));
  const named = $derived(client.name || 'an app');

  async function load() {
    if (state === 'error') return;
    if (!request) return fail('This page needs a request from the app you are connecting. Go back to it and connect again.');
    try {
      const out = await api(`/auth/oauth/request?request=${encodeURIComponent(request)}`);
      client = out.client;
      user = out.user;
      archives = out.workspaces.map((w) => ({ ...w, look: lookOf(w) }));
      if (!archives.some((w) => w.id === picked)) picked = (archives.find((w) => w.id === workspace()) ?? archives[0])?.id ?? '';
      state = !user ? 'signin' : archives.length ? 'pick' : 'none';
    } catch (e) {
      fail(e.message);
    }
  }

  function fail(message) {
    error = message.replace(/: (bad request|forbidden|unauthorized)$/, '.');
    state = 'error';
  }

  async function allow() {
    busy = true;
    try {
      const out = await api('/auth/oauth/answer', { method: 'POST', body: { request, workspace: picked, allow: true } });
      state = 'done';
      location.assign(out.redirect);
    } catch (e) {
      fail(e.message);
    }
    busy = false;
  }

  async function switchAccount() {
    await signOut();
    user = null;
    state = 'signin';
  }

  load();
</script>

{#if state === 'loading'}
  <Loading />
{:else if state === 'signin'}
  <SignIn title={`Sign in to connect ${named}`} art="authorize" next={location.pathname + location.search} done={load} />
{:else}
  <div class="screen" transition:fade={{ speed: 'normal' }}>
    <div class="card">
      <Art name="authorize" size={state === 'pick' ? 'md' : 'lg'} />
      {#if state === 'pick'}
        <Heading level={1} size="xl" leading="inherit" wrap="balance">Connect {named} to Archive</Heading>
        <div class="archives" role="radiogroup" aria-label="Archive">
          {#each archives as w (w.id)}
            <label class={['archive', { on: w.id === picked }]}>
              <input type="radio" name="archive" value={w.id} bind:group={picked} />
              <ArchiveIcon icon={w.look.icon} tint={w.look.tint} seed={w.id} size="lg" active={w.id === picked} />
              <span class="name">
                <Text weight="medium" truncate>{w.name}</Text>
                <Text size="sm" tone="tertiary">{TYPES[w.look.type]?.label ?? 'Archive'}</Text>
              </span>
              <span class="tick"><Icon icon={Check} size="sm" /></span>
            </label>
          {/each}
        </div>
        <Button variant="primary" size="lg" full loading={busy} disabled={!picked} onclick={allow}>Allow</Button>
        <Text as="p" size="sm" tone="tertiary">
          Signed in as {user.email}. <button class="link" onclick={switchAccount}>Not you?</button>
        </Text>
      {:else if state === 'none'}
        <Heading level={1} size="xl" leading="inherit" wrap="normal">No archive to connect</Heading>
        <Text as="p" tone="secondary">You aren’t in any archive yet. Ask an owner to add you, then connect {named} again.</Text>
        <Button variant="ghost" onclick={switchAccount}>Use another account</Button>
      {:else if state === 'done'}
        <Heading level={1} size="xl" leading="inherit" wrap="normal">Connected to {chosen?.name}</Heading>
        <Text as="p" tone="secondary">Taking you back to {client.name || 'the app'}. If nothing happens, you can close this tab.</Text>
      {:else}
        <Heading level={1} size="xl" leading="inherit" wrap="normal">That didn’t connect</Heading>
        <p class="error" role="alert">{error}</p>
      {/if}
    </div>
  </div>
{/if}

<style>
  .screen {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    padding: var(--space-6);
    overflow: auto;
    background: var(--color-canvas);
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-5);
    width: 100%;
    max-width: var(--measure-xs);
    text-align: center;
  }


  .archives {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    width: 100%;
    max-height: calc(var(--control-xl) * 6);
    padding: var(--space-1);
    overflow-y: auto;
    border-radius: var(--radius-xl);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
    text-align: start;
  }

  .archive {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3) var(--space-2) var(--space-2);
    border-radius: var(--radius-lg);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out);
  }

  /* Picked as a sidebar item is active: a neutral fill, a stronger one when picked. */
  .archive:hover {
    background: var(--color-fill);
  }

  .archive.on,
  .archive.on:hover {
    background: var(--color-fill-strong);
  }

  .archive:has(input:focus-visible) {
    outline: var(--focus-width) solid var(--color-focus);
    outline-offset: calc(var(--border-width) * -1);
  }

  .archive input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  .name {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }

  .tick {
    display: grid;
    place-items: center;
    width: var(--avatar-xs);
    height: var(--avatar-xs);
    border-radius: 50%;
    color: var(--color-text-inverse);
    background: var(--color-inverse);
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-out);
  }

  .on .tick {
    opacity: 1;
  }






  .link {
    color: var(--color-accent);
    font: inherit;
  }

  .link:hover {
    text-decoration: underline;
  }

  .error {
    color: var(--color-danger);
    font-size: var(--text-sm);
  }
</style>
