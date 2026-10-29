<script>
  import Icon from '#kit/Icon.svelte';
  import { go } from '../api.js';

  let { columns, rows, cell, td, key: rowKey = (r) => r.id, sort = null, onsort, fill = false, href, target } = $props();

  function open(r, e) {
    const url = href(r);
    if (target || e.metaKey || e.ctrlKey || e.shiftKey || url.startsWith('/api/')) window.open(url, target || '_blank', 'noopener');
    else go(url);
  }

  function press(r, e) {
    if (e.key === 'Enter') open(r, e);
  }
</script>

<div class="wrap" class:fill>
  <table>
    <thead>
      <tr>
        {#each columns as c (c.name)}
          {@const on = sort?.col === c.name}
          <th class:label={c.label} aria-sort={on ? (sort.desc ? 'descending' : 'ascending') : undefined}>
            {#if onsort}
              <button type="button" class:on onclick={() => onsort(c.name)}>
                {c.label ?? c.name.replaceAll('_', ' ')}{#if on}<Icon name={sort.desc ? 'arrow-down' : 'arrow-up'} />{/if}
              </button>
            {:else}
              {c.label ?? c.name.replaceAll('_', ' ')}
            {/if}
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as r, i (rowKey(r, i))}
        <tr class:link={href} role={href ? 'link' : undefined} tabindex={href ? 0 : undefined} onclick={href ? (e) => open(r, e) : undefined} onkeydown={href ? (e) => press(r, e) : undefined}>
          {#each columns as c (c.name)}
            <td class:ref={c.ref}>{#if td}{@render td(r, c)}{:else}{cell(r[c.name])}{/if}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap { overflow-x: auto; margin-inline: calc(-1 * var(--space-2)); }
  .wrap.fill { flex: 1 1 auto; min-height: 0; margin: calc(-1 * var(--table-bleed)) calc(-1 * var(--table-bleed)) 0; overflow: auto; }
  .fill table { font-size: var(--text-table); }
  .fill th, .fill td { padding: var(--space-2); }
  .fill th { position: sticky; top: 0; z-index: var(--z-raised); background: var(--surface-band); color: var(--fg-muted); font-weight: var(--weight-medium); line-height: var(--leading-none); }
  table { min-width: 100%; border-collapse: collapse; font-size: var(--text-table); line-height: var(--leading-medium); }
  th {
    padding: var(--space-2);
    color: var(--fg-muted);
    font-weight: var(--weight-normal);
    text-align: left;
    text-transform: capitalize;
    white-space: nowrap;
  }
  td {
    max-width: var(--cell-width);
    padding: var(--space-2);
    overflow: hidden;
    border-top: var(--hairline) solid var(--fill);
    color: var(--fg-soft);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .label { text-transform: none; }
  th button { display: inline-flex; align-items: center; gap: var(--space-1); color: inherit; text-transform: inherit; white-space: nowrap; transition: color var(--duration-fast); }
  th button:hover, th .on { color: var(--fg); }
  th button :global(.icon) { width: var(--text-table); height: var(--text-table); color: var(--fg-muted); }
  td:first-child { color: var(--fg); font-weight: var(--weight-medium); }
  tbody tr:hover td { background: var(--fill-faint); }
  .link { cursor: pointer; outline: 0; }
  .link:focus-visible td { background: var(--fill); }
  .ref { color: var(--fg-muted); font-family: var(--font-mono); }
</style>
