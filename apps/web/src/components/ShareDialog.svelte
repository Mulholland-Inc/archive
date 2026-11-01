<script>
  import Dialog from './Dialog.svelte';
  import PeoplePicker from './PeoplePicker.svelte';
  import Button from './Button.svelte';
  import SharePerson from './SharePerson.svelte';
  import { route } from '../lib/router.svelte.js';
  import { app, shareItem, shareFor, addPeople, contacts } from '../lib/app.svelte.js';

  let picked = $state([]);

  const kind = $derived(app.share.kind);
  const target = $derived(app.share.target);
  const item = $derived(kind ? shareItem(kind, target) : null);
  const share = $derived(kind ? shareFor(kind, target) : null);
  const suggestions = $derived(kind ? contacts().filter((p) => !share?.people.some((x) => x.email === p.email)) : []);

  $effect(() => {
    route.path;
    app.share.open = false;
  });

  function add(event) {
    event?.preventDefault();
    if (!picked.length) return;
    addPeople(kind, target, picked.map((p) => p.email), 'view');
    picked = [];
  }
</script>

<Dialog bind:open={app.share.open} title={item ? `Share “${item.name}”` : 'Share'}>
  {#if item}
    <form class="add" onsubmit={add}>
      <div class="field"><PeoplePicker bind:value={picked} people={suggestions} label="Add people" /></div>
      <Button type="submit" variant="primary" disabled={!picked.length}>Share</Button>
    </form>
    {#if share?.people.length}
      <section class="people">
        {#each share.people as person (person.email)}
          <SharePerson {person} {share} />
        {/each}
      </section>
    {/if}
  {/if}
  {#snippet footer({ close })}
    <Button variant="primary" onclick={close}>Done</Button>
  {/snippet}
</Dialog>

<style>
  .add {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: var(--space-2);
  }

  .field {
    flex: 1 1 var(--col-md);
    min-width: 0;
  }

  .people {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
</style>
