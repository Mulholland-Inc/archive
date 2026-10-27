<script>
  import { tick } from 'svelte';
  import { hidden } from './api.js';
  import Bar from './ui/Bar.svelte';
  import Chip from './ui/Chip.svelte';
  import Entity from './ui/Entity.svelte';
  import Grid from './ui/Grid.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Row from './ui/Row.svelte';
  import Search from './ui/Search.svelte';

  let { tables, views, types } = $props();
  let query = $state('');
  let lit = $state('');
  let open = $state({});
  let timer;

  const own = (t) => t.columns.filter((c) => !hidden.has(c.name));
  const byName = $derived(Object.fromEntries([...tables, ...views].map((t) => [t.name, t])));
  const sources = $derived(
    Object.fromEntries(
      views.map((v) => {
        const names = own(v).map((c) => c.name);
        let best, score = 0;
        for (const t of tables) {
          const have = new Set(own(t).map((c) => c.name));
          const n = names.filter((x) => have.has(x)).length;
          if (n > score) [best, score] = [t, n];
        }
        return [v.name, score * 2 > names.length ? best : undefined];
      }),
    ),
  );
  const columns = (t) => {
    const from = sources[t.name];
    return own(t).map((c) => {
      const s = from?.columns.find((x) => x.name === c.name && x.type === c.type);
      return s ? { ...c, ref: c.ref || s.ref, references: c.references || s.references } : c;
    });
  };
  const target = (c) => c.references || c.ref.split('.').pop();
  const incoming = $derived.by(() => {
    const m = {};
    for (const t of tables) for (const c of own(t)) if (byName[target(c)]) (m[target(c)] ??= []).push({ from: t, column: c.name });
    return m;
  });
  const shown = (t) => t.kind === 'view' ? `${t.layout || 'table'}${t.by ? ` by ${t.by}` : ''}` : `${own(t).length} columns`;
  const q = $derived(query.trim().toLowerCase());
  const hit = (t) => !q || t.name.includes(q) || own(t).some((c) => c.name.includes(q) || c.type.includes(q));
  const used = $derived.by(() => {
    const keys = new Set([...tables, ...views].flatMap((t) => own(t).map((c) => `${c.schema}.${c.type}`)));
    return Object.values(types).filter((t) => keys.has(`${t.schema}.${t.name}`)).sort((a, b) => a.name.localeCompare(b.name));
  });
  const groups = $derived(
    [
      { kind: 'e', title: 'Enums', icon: 'list' },
      { kind: 'c', title: 'Composites', icon: 'braces' },
      { kind: 'd', title: 'Domains', icon: 'type' },
    ].map((g) => ({ ...g, types: used.filter((t) => t.kind === g.kind && (!q || t.name.includes(q) || t.enum.some((v) => v.includes(q)))) })).filter((g) => g.types.length),
  );
  const sections = $derived([
    { title: 'Tables', items: tables.filter(hit) },
    { title: 'Views', items: views.filter(hit) },
  ].filter((s) => s.items.length));
  const detail = (t) => t?.kind === 'e' ? t.enum.join(', ') : t?.kind === 'c' ? t.attrs.map((a) => `${a.name} ${a.type}`).join(', ') : t?.base;

  async function focus(name) {
    if (!hit(byName[name])) query = '';
    lit = name;
    await tick();
    document.getElementById(`ontology-${name}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    clearTimeout(timer);
    timer = setTimeout(() => (lit = ''), 1600);
  }
</script>

{#snippet kind(t, c)}
  {@const n = target(c)}
  {@const ty = types[`${c.schema}.${c.type}`]}
  {@const list = c.list ? '[]' : ''}
  {#if byName[n]}
    <Chip icon={byName[n].icon} onclick={() => focus(n)} title="Show {n}">{n}{list}</Chip>
  {:else if n === 'file'}
    <Chip icon="file" href="/files">file{list}</Chip>
  {:else if ty?.kind === 'e'}
    {@const key = `${t.name}.${c.name}`}
    <Chip icon="list" active={open[key]} onclick={() => (open[key] = !open[key])} title={ty.enum.join(', ')} aria-expanded={!!open[key]}>{c.type}{list}</Chip>
  {:else}
    <span title={detail(ty)}>{c.type}{list}</span>
  {/if}
{/snippet}

<Heading title="Ontology" icon="waypoints" />
<Bar>
  <Search bind:value={query} placeholder="Filter" aria-label="Filter tables and columns" />
</Bar>
<div class="sections">
  {#each sections as s (s.title)}
    <section>
      <h2>{s.title}</h2>
      <Grid>
        {#each s.items as t (t.name)}
          <Entity id="ontology-{t.name}" title={t.name} icon={t.icon} href={t.kind === 'view' ? `/views/${t.name}` : `/${t.name}`} lit={lit === t.name}>
            {#snippet aside()}<span class="tag">{shown(t)}</span>{/snippet}
            {#if sources[t.name]}
              <Row label="reads"><Chip icon={sources[t.name].icon} onclick={() => focus(sources[t.name].name)}>{sources[t.name].name}</Chip></Row>
            {/if}
            {#each columns(t) as c (c.name)}
              {@const ty = types[`${c.schema}.${c.type}`]}
              <Row label={c.name} mark={c.not_null && !c.default && !c.generated} items={open[`${t.name}.${c.name}`] ? ty.enum : []}>{@render kind(t, c)}</Row>
            {/each}
            {#if incoming[t.name]}
              <Row label="referenced by" wide>
                {#each incoming[t.name] as r (`${r.from.name}.${r.column}`)}
                  <Chip icon={r.from.icon} onclick={() => focus(r.from.name)}>{r.from.name}.{r.column}</Chip>
                {/each}
              </Row>
            {/if}
            {#if views.some((v) => sources[v.name] === t)}
              <Row label="views" wide>
                {#each views.filter((v) => sources[v.name] === t) as v (v.name)}
                  <Chip icon={v.icon} onclick={() => focus(v.name)}>{v.name}</Chip>
                {/each}
              </Row>
            {/if}
          </Entity>
        {/each}
      </Grid>
    </section>
  {/each}
  {#if groups.length}
    <section>
      <h2>Types</h2>
      <Grid>
        {#each groups as g (g.kind)}
          <Entity title={g.title} icon={g.icon}>
            {#snippet aside()}<span class="tag">{g.types.length}</span>{/snippet}
            {#each g.types as t (t.name)}
              {#if g.kind === 'd'}
                <Row label={t.name}>{t.base}</Row>
              {:else}
                <Row label={t.name} items={g.kind === 'e' ? t.enum : t.attrs.map((a) => `${a.name} ${a.type}${a.list ? '[]' : ''}`)} />
              {/if}
            {/each}
          </Entity>
        {/each}
      </Grid>
    </section>
  {/if}
  {#if !sections.length && !groups.length}<Note>Nothing matches.</Note>{/if}
</div>

<style>
  .sections { display: flex; flex-direction: column; gap: var(--space-6); }
  section { display: flex; flex-direction: column; gap: var(--space-4); }
  h2 { font: var(--weight-medium) var(--display-sm)/var(--leading-tight) var(--display); letter-spacing: var(--tracking-tight); }
  .tag { flex: none; color: var(--text-3); font: var(--text-xs)/1 var(--mono); }
</style>
