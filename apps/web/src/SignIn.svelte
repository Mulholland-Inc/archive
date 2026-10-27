<script>
  import { api } from './api.js';
  import Logo from './Logo.svelte';

  let email = $state('');
  let sent = $state(false);
  let error = $state('');

  async function send(e) {
    e.preventDefault();
    error = '';
    try {
      await api('/auth/magic', { method: 'POST', body: { email } });
      sent = true;
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="door">
  <div class="brand"><Logo height={24} /></div>
  <h1>Hello</h1>
  <p class="lead">{sent ? `If ${email} has an account, a sign-in link is on its way.` : 'Sign in to your archive.'}</p>
  {#if !sent}
    <form onsubmit={send}>
      <input type="email" placeholder="Sign in with your email" bind:value={email} required autofocus />
      <button aria-label="Send sign-in link">→</button>
    </form>
    {#if error}<p class="error">{error}</p>{/if}
  {/if}
</div>

<style>
  .door { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; padding: 24px; text-align: center; }
  .brand { position: absolute; top: 24px; left: 32px; }
  h1 { font-size: 96px; }
  form { position: relative; width: 100%; max-width: 520px; margin-top: 40px; }
  input { width: 100%; padding-right: 64px; }
  form button { position: absolute; right: 8px; top: 8px; width: 40px; height: 40px; padding: 0; justify-content: center; font-size: 18px; }
  .error { color: var(--alarm); }
</style>
