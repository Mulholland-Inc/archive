<script>
  import { api } from './api.js';
  import Button from './ui/Button.svelte';
  import Field from './ui/Field.svelte';
  import Heading from './ui/Heading.svelte';
  import Logo from './ui/Logo.svelte';
  import Note from './ui/Note.svelte';

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
  <div class="brand"><Logo large /></div>
  <Heading hero title="Hello" description={sent ? `If ${email} has an account, a sign-in link is on its way.` : 'Sign in to your archive.'} />
  {#if !sent}
    <form onsubmit={send}>
      <Field type="email" placeholder="Sign in with your email" bind:value={email} required autofocus>
        {#snippet action()}<Button icon="arrow-right" aria-label="Send sign-in link" />{/snippet}
      </Field>
    </form>
    {#if error}<Note error>{error}</Note>{/if}
  {/if}
</div>

<style>
  .door { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; padding: var(--space-5); text-align: center; }
  .brand { position: absolute; top: var(--space-5); left: var(--space-6); }
  form { width: 100%; max-width: var(--form); margin: var(--space-7) 0 var(--space-4); }
</style>
