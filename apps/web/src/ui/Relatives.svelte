<script>
  import { loadBook } from './FamilyTree.svelte';
  import ListItem from './ListItem.svelte';

  let { view, id } = $props();
  let book = $state(null);

  const partnerOf = (u) => (u.a === id ? u.b : u.a);
  const groups = $derived.by(() => {
    if (!book?.people[id]) return [];
    const me = book.people[id];
    const born = me.parents.map((u) => book.unions[u]);
    const made = me.unions.map((u) => book.unions[u]);
    const once = (ids) => [...new Set(ids.filter((p) => p && p !== id))];
    return [
      ['Parents', once(born.flatMap((u) => [u.a, u.b]))],
      ['Partners', once(made.map(partnerOf))],
      ['Children', once(made.flatMap((u) => u.kids))],
      ['Brothers and sisters', once(born.flatMap((u) => u.kids))],
    ].filter(([, ids]) => ids.length);
  });

  loadBook(view).then((b) => (book = b.book)).catch(() => {});
</script>

{#each groups as [title, ids] (title)}
  <section>
    <h3>{title}</h3>
    {#each ids as p (p)}
      <ListItem href="/person/{p}" icon="user" meta={[book.people[p].years]}>{book.people[p].name}</ListItem>
    {/each}
  </section>
{/each}

<style>
  section { display: flex; flex-direction: column; gap: var(--space-1); margin-top: var(--space-5); }
  h3 { margin-bottom: var(--space-1); color: var(--fg-muted); font-size: var(--text-meta); font-weight: var(--weight-medium); }
</style>
