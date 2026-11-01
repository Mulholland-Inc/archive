<script>
  import { Link, Globe, Lock } from '@lucide/svelte';
  import Dialog from './Dialog.svelte';
  import Input from './Input.svelte';
  import Select from './Select.svelte';
  import Button from './Button.svelte';
  import SharePerson from './SharePerson.svelte';
  import { me as meUser } from '../lib/data.js';
  import { route } from '../lib/router.svelte.js';
  import { app, ws, permissions, shareItem, shareFor, addPeople, setLinkAccess, copyLink } from '../lib/app.svelte.js';

  let emails = $state('');
  let permission = $state('view');

  const kind = $derived(app.share.kind);
  const target = $derived(app.share.target);
  const item = $derived(kind ? shareItem(kind, target) : null);
  const share = $derived(kind ? shareFor(kind, target) : null);
  const list = $derived(emails.split(/[\s,;]+/).filter((e) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(e)));
  const me = $derived({ name: app.profile.name, email: ws().members.find((m) => m.name === app.profile.name)?.email ?? '', user: meUser.id });
  const general = $derived([
    { value: 'off', label: 'Restricted', icon: Lock },
    ...(ws().settings.sharing || share?.link === 'view' ? [{ value: 'view', label: 'Anyone with the link can view', icon: Globe }] : [])
  ]);

  $effect(() => {
    route.path;
    app.share.open = false;
  });

  function add(event) {
    event?.preventDefault();
    if (!list.length) return;
    addPeople(kind, target, list, permission);
    emails = '';
  }
</script>

<Dialog bind:open={app.share.open} title={item ? `Share “${item.name}”` : 'Share'}>
  {#if item}
    <form class="add" onsubmit={add}>
      <div class="email"><Input bind:value={emails} placeholder="Add people by email" aria-label="Email addresses" /></div>
      <Select bind:value={permission} options={permissions} variant="ghost" label="Permission" placement="bottom-end" />
      <Button type="submit" variant="primary" disabled={!list.length}>Share</Button>
    </form>
    <section class="block">
      <h3>People with access</h3>
      <SharePerson person={me} fixed="Owner" />
      {#each share?.people ?? [] as person (person.email)}
        <SharePerson {person} {share} />
      {/each}
    </section>
    <section class="block">
      <h3>General access</h3>
      <Select value={share?.link ?? 'off'} options={general} variant="ghost" label="General access" onchange={(v) => setLinkAccess(kind, target, v)} />
    </section>
  {/if}
  {#snippet footer({ close })}
    <Button icon={Link} onclick={() => item && copyLink(item.path)}>Copy link</Button>
    <Button variant="primary" onclick={close}>Done</Button>
  {/snippet}
</Dialog>

<style>
  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
  }

  .email {
    flex: 1 1 var(--col-md);
    min-width: 0;
  }

  .email :global(.box) {
    width: 100%;
  }

  .block {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  h3 {
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }
</style>
