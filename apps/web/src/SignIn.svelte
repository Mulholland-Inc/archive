<script module>
  const bytes = (s) => Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(s.length / 4) * 4, '=')), (c) => c.charCodeAt(0));
  const text = (b) => btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
</script>

<script>
  import { api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Word from '#kit/Word.svelte';
  import art from '../../site/static/art/cli.webp';

  let { oncancel, create = false } = $props();
  let team = $state('');
  let busy = $state(false);
  let error = $state('');
  let providers = $state([]);
  let field = $state();
  const passkeys = typeof PublicKeyCredential !== 'undefined';
  const name = (p) => p.charAt(0).toUpperCase() + p.slice(1);

  api('/auth/methods')
    .then((m) => (providers = m.providers ?? []))
    .catch(() => {});

  $effect(() => {
    field?.focus();
  });

  async function make(e) {
    e.preventDefault();
    if (busy || !team.trim()) return;
    busy = true;
    error = '';
    try {
      const made = await api('/auth/signup', { method: 'POST', body: { name: team.trim() } });
      location.assign('/?workspace=' + encodeURIComponent(made.workspace));
    } catch (err) {
      error = err.message;
    }
    busy = false;
  }

  async function passkey() {
    error = '';
    try {
      const opts = await api('/auth/passkey/login', { method: 'POST' });
      const pk = opts.publicKey;
      pk.challenge = bytes(pk.challenge);
      pk.allowCredentials = (pk.allowCredentials ?? []).map((c) => ({ ...c, id: bytes(c.id) }));
      const cred = await navigator.credentials.get({ publicKey: pk });
      const r = cred.response;
      await api('/auth/passkey/login/finish', {
        method: 'POST',
        body: {
          id: cred.id,
          rawId: text(cred.rawId),
          type: cred.type,
          response: { clientDataJSON: text(r.clientDataJSON), authenticatorData: text(r.authenticatorData), signature: text(r.signature), userHandle: r.userHandle ? text(r.userHandle) : null }
        }
      });
      location.assign('/');
    } catch (err) {
      if (err.name !== 'NotAllowedError' && err.name !== 'AbortError') error = err.message;
    }
  }
</script>

<div class="door">
  <span class="brand" aria-label="Archive"><Word /></span>
  <div class="column">
    <img src={art} alt="" />
    {#if create}
      <form class="pill" onsubmit={make}>
        <input bind:this={field} bind:value={team} placeholder="Workspace name" aria-label="Workspace name" autocomplete="organization" required />
        <button class="go" type="submit" disabled={busy || !team.trim()} aria-label="Create workspace">
          <span class:spin={busy}><Icon name={busy ? 'loader-circle' : 'arrow-up'} /></span>
        </button>
      </form>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if !create && (providers.length || passkeys)}
      <div class="methods">
        {#each providers as p (p)}
          <a class="method" href="/auth/oidc/{encodeURIComponent(p)}">Continue with {name(p)}</a>
        {/each}
        {#if passkeys}<button class="method" type="button" onclick={passkey}><Icon name="key-round" />Use a passkey</button>{/if}
      </div>
    {/if}
    {#if oncancel}<button class="again" type="button" onclick={oncancel}>Back</button>{/if}
  </div>
</div>

<style>
  .door {
    display: flex;
    flex-direction: column;
    min-height: calc(100svh - 2 * var(--frame) - env(safe-area-inset-top));
  }
  .brand { display: flex; align-self: flex-start; align-items: center; min-height: var(--size-control-lg); padding: var(--item-pad); color: var(--fg); }
  .brand :global(.word) { height: var(--text-lg); width: auto; fill: currentColor; }
  .column {
    display: flex;
    flex: 1;
    flex-direction: column;
    justify-content: center;
    gap: var(--space-4);
    width: 100%;
    max-width: var(--door-width);
    align-self: center;
    padding: var(--gutter);
    animation: appear var(--duration-slow) var(--ease-out) both;
  }
  img { align-self: center; width: auto; height: var(--door-art); max-width: 100%; object-fit: contain; }
  .pill { display: flex; align-items: center; gap: var(--space-1); padding: var(--space-1-5); border-radius: var(--radius-full); background: var(--composer-bg); }
  input { flex: 1; min-width: 0; padding: var(--composer-pad); background: none; color: var(--fg); line-height: var(--leading-medium); outline: 0; }
  input::placeholder { color: var(--fg-muted); }
  .go {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--composer-line);
    height: var(--composer-line);
    border-radius: var(--radius-full);
    background: var(--primary);
    color: var(--on-primary);
    transition: background var(--duration-fast), opacity var(--duration-fast), transform var(--duration-fast) var(--ease-out);
  }
  .go:hover { background: var(--primary-hover); }
  .go:active { transform: scale(var(--scale-press)); }
  .go:disabled { opacity: var(--opacity-dim); }
  .spin { display: grid; animation: spin var(--duration-loop) linear infinite; }
  .methods { display: flex; flex-direction: column; gap: var(--space-2); }
  .method {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-full);
    background: var(--fill);
    color: var(--fg);
    transition: background var(--duration-fast);
  }
  .method:hover { background: var(--fill-strong); }
  .again { align-self: center; color: var(--accent); }
  .again:hover { text-decoration: underline; }
  .error { color: var(--danger); text-align: center; }
  @keyframes appear { from { opacity: 0; } }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) {
    .column { animation: none; }
  }
</style>
