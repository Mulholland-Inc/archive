<script>
  import { getContext } from 'svelte';
  import { api, hidden, show } from './api.js';
  import Access from './ui/Access.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Relatives from './ui/Relatives.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { table, id, tree = null } = $props();
  let row = $state(null);
  let error = $state('');

  const label = (n) => n.replaceAll('_', ' ');
  const present = (v) => v !== null && v !== undefined && v !== '' && v !== false && !(Array.isArray(v) && !v.length);
  const sorter = (c) => c.name.endsWith('_year') && present(row[c.name.slice(0, -5)]);
  const quiet = (c) => c === head || (tree && ['home', 'sex'].includes(c.name));
  const head = $derived(row ? table.columns.find((c) => !hidden.has(c.name) && !c.ref && typeof row[c.name] === 'string' && row[c.name]) : null);
  const fields = $derived(row ? table.columns.filter((c) => !hidden.has(c.name) && present(row[c.name]) && !sorter(c) && !quiet(c)) : []);
  const title = $derived(head ? row[head.name] : '');
  const link = (c) => `/${c.ref.split('.').pop()}/${row[c.name]}`;
  const ctx = getContext('access');
  const access = $derived(table.columns.find((c) => c.name === 'access'));
  const fixed = $derived(!!access?.not_null);

  async function setAccess(v) {
    try {
      row = await api(`/api/${table.name}/${id}`, { method: 'PATCH', body: { access: v } });
      error = '';
    } catch (e) {
      error = e.message;
      row = { ...row };
    }
  }

  api(`/api/${table.name}/${id}`).then((r) => (row = r)).catch((e) => (error = e.message));
</script>

<Heading plain title={title || label(table.name)} />
<Sheet>
  {#if error}<Note error>{error}</Note>{/if}
  {#if row}
    <dl>
      {#each fields as c (c.name)}
        <dt>{label(c.name)}</dt>
        <dd>
          {#if c.ref && !c.list && c.ref.startsWith('public.')}<a href={link(c)}>{label(c.ref.split('.').pop())}</a>{:else}{row[c.name] === true ? 'Yes' : show(row[c.name])}{/if}
        </dd>
      {/each}
      {#if access && (row.access || (!fixed && ctx.role !== 'viewer' && ctx.groups.length))}
        <dt>access</dt>
        <dd>{#key row}<Access value={row.access} {fixed} onpick={setAccess} />{/key}</dd>
      {/if}
    </dl>
    {#if tree}<Relatives view={tree} {id} />{/if}
  {/if}
</Sheet>

<style>
  dl { display: grid; grid-template-columns: minmax(96px, max-content) minmax(0, 1fr); gap: var(--space-2) var(--space-5); margin: 0; }
  dt { color: var(--fg-muted); text-transform: capitalize; }
  dd { margin: 0; overflow-wrap: anywhere; white-space: pre-line; }
  dd a { text-decoration: underline; text-transform: capitalize; }
</style>
