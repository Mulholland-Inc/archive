<script>
  import { api, devicePasskey, passkeyLogin, passkeys } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import { BRANDS } from '#kit/brands.js';
  import Word from '#kit/Word.svelte';
  import art from '../../site/static/art/cli.webp';

  let error = $state('');
  let providers = $state([]);
  const first = passkeys && devicePasskey();

  api('/auth/methods')
    .then((m) => (providers = m.providers ?? []))
    .catch(() => {});

  async function passkey() {
    error = '';
    try {
      await passkeyLogin();
      location.assign('/');
    } catch (err) {
      if (err.name !== 'NotAllowedError' && err.name !== 'AbortError') error = err.message;
    }
  }
</script>

{#snippet key()}
  <button class={['method', first ? 'primary' : 'quiet']} type="button" onclick={passkey}><Icon name="key-round" />Use a passkey</button>
{/snippet}

<div class="door">
  <span class="brand" aria-label="Archive"><Word /></span>
  <div class="column">
    <img src={art} alt="" />
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if providers.length || passkeys}
      <div class="methods">
        {#if first}{@render key()}{/if}
        {#each providers as p (p.id)}
          <a class="method" href="/auth/oidc/{encodeURIComponent(p.id)}"
            >{#if BRANDS[p.name]}<svg class="icon filled" viewBox={BRANDS[p.name][0]} aria-hidden="true">{@html BRANDS[p.name][1]}</svg>{/if}Continue with {p.name}</a
          >
        {/each}
        {#if passkeys && !first}{@render key()}{/if}
      </div>
    {/if}
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
    transition: background var(--duration-fast), color var(--duration-fast);
  }
  .method:hover { background: var(--fill-strong); }
  .primary { background: var(--primary); color: var(--on-primary); }
  .primary:hover { background: var(--primary-hover); }
  .quiet { background: none; color: var(--fg-muted); }
  .quiet:hover { background: var(--fill-faint); color: var(--fg); }
  .error { color: var(--danger); text-align: center; }
  @keyframes appear { from { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) {
    .column { animation: none; }
  }
</style>
