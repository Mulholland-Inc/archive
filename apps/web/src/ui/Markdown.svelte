<script module>
  const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

  export function inline(s) {
    const codes = [];
    s = s.replace(/`([^`]+)`/g, (_, c) => `\u0000${codes.push(c) - 1}\u0000`);
    s = esc(s)
      .replace(/\[([^\]]+)\]\(((?:https?:\/\/|\/)[^\s)]+)\)/g, (_, t, href) => `<a href="${href}"${href.startsWith('/') ? '' : ' target="_blank" rel="noreferrer"'}>${t}</a>`)
      .replace(/(^|[\s(])(https?:\/\/[^\s<)]+[^\s<).,;:!?])/g, '$1<a href="$2" target="_blank" rel="noreferrer">$2</a>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/__([^_]+)__/g, '<strong>$1</strong>')
      .replace(/~~([^~]+)~~/g, '<del>$1</del>')
      .replace(/(^|[^*\w])\*([^*\n]+)\*(?!\w)/g, '$1<em>$2</em>')
      .replace(/(^|[^_\w])_([^_\n]+)_(?!\w)/g, '$1<em>$2</em>');
    return s.replace(/\u0000(\d+)\u0000/g, (_, i) => `<code>${esc(codes[i])}</code>`);
  }

  const cells = (row) => row.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim());
  const isRow = (l) => /^\s*\|.*\|\s*$/.test(l ?? '');
  const isRule = (l) => /^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$/.test(l ?? '');
  const isItem = (l) => /^\s*([-*+]|\d+[.)])\s/.test(l);
  const starts = /^(```|#{1,6}\s|>\s?|\s*([-*+]|\d+[.)])\s|\s*(---|\*\*\*)\s*$)/;

  export function blocks(src) {
    const lines = src.replace(/\r/g, '').split('\n');
    const out = [];
    for (let i = 0; i < lines.length; ) {
      const line = lines[i];
      if (/^\s*```/.test(line)) {
        const body = [];
        for (i++; i < lines.length && !/^\s*```/.test(lines[i]); i++) body.push(lines[i]);
        out.push({ type: 'code', code: body.join('\n'), open: i >= lines.length });
        i++;
      } else if (/^#{1,6}\s/.test(line)) {
        out.push({ type: 'heading', html: inline(line.replace(/^#+\s*/, '')) });
        i++;
      } else if (isRow(line) && isRule(lines[i + 1])) {
        const head = cells(line);
        const rows = [];
        for (i += 2; i < lines.length && /^\s*\|/.test(lines[i]); i++) {
          const c = cells(lines[i]);
          rows.push({ html: Object.fromEntries(head.map((h, j) => [`c${j}`, inline(c[j] ?? '')])), text: Object.fromEntries(head.map((h, j) => [`c${j}`, (c[j] ?? '').replace(/[*_`~]/g, '').trim()])) });
        }
        out.push({ type: 'table', columns: head.map((h, j) => ({ name: `c${j}`, label: h.replace(/[*_`]/g, '') })), rows });
      } else if (/^>\s?/.test(line)) {
        const body = [];
        for (; i < lines.length && /^>\s?/.test(lines[i]); i++) body.push(lines[i].replace(/^>\s?/, ''));
        out.push({ type: 'quote', html: body.map(inline).join('<br>') });
      } else if (isItem(line)) {
        const ordered = /^\s*\d/.test(line);
        const items = [];
        for (; i < lines.length && (isItem(lines[i]) || (/^\s{2,}\S/.test(lines[i]) && items.length)); i++) {
          if (isItem(lines[i])) items.push({ html: inline(lines[i].replace(/^\s*([-*+]|\d+[.)])\s+/, '')), nested: /^\s{2,}/.test(lines[i]) });
          else items.at(-1).html += ' ' + inline(lines[i].trim());
        }
        out.push({ type: ordered ? 'ol' : 'ul', items, start: ordered ? parseInt(line) || 1 : 1 });
      } else if (/^\s*(---|\*\*\*)\s*$/.test(line)) {
        out.push({ type: 'rule' });
        i++;
      } else if (!line.trim()) {
        i++;
      } else {
        const para = [];
        for (; i < lines.length && lines[i].trim() && !starts.test(lines[i]) && !(isRow(lines[i]) && isRule(lines[i + 1])); i++) para.push(lines[i]);
        if (!para.length) para.push(lines[i++]);
        out.push({ type: 'p', html: para.map(inline).join('<br>') });
      }
    }
    return out;
  }
</script>

<script>
  import CodeBlock from './CodeBlock.svelte';
  import ChatTable from './ChatTable.svelte';

  let { text } = $props();
  const parts = $derived(blocks(text));
</script>

<div class="markdown">
  {#each parts as b, i (i)}
    {#if b.type === 'p'}
      <p>{@html b.html}</p>
    {:else if b.type === 'heading'}
      <h3>{@html b.html}</h3>
    {:else if b.type === 'code'}
      <CodeBlock code={b.code} live={b.open} />
    {:else if b.type === 'table'}
      <ChatTable columns={b.columns} rows={b.rows} />
    {:else if b.type === 'quote'}
      <blockquote>{@html b.html}</blockquote>
    {:else if b.type === 'ul'}
      <ul>
        {#each b.items as it, j (j)}<li class:nested={it.nested}>{@html it.html}</li>{/each}
      </ul>
    {:else if b.type === 'ol'}
      <ol start={b.start}>
        {#each b.items as it, j (j)}<li class:nested={it.nested}>{@html it.html}</li>{/each}
      </ol>
    {:else if b.type === 'rule'}
      <hr />
    {/if}
  {/each}
</div>

<style>
  .markdown { display: flex; flex-direction: column; gap: var(--space-3); min-width: 0; color: var(--fg); font-size: var(--text-chat); line-height: var(--leading-ui); overflow-wrap: anywhere; }
  h3 { font-weight: var(--weight-semibold); }
  ul, ol { display: flex; flex-direction: column; gap: var(--space-1); list-style-position: inside; }
  li::marker { color: var(--fg-muted); }
  .nested { list-style-type: circle; }
  blockquote { padding: var(--space-3); border-radius: var(--radius-item); background: var(--fill-faint); color: var(--fg-soft); }
  hr { height: var(--hairline); border: 0; background: var(--fill); }
  .markdown :global(:not(pre) > code) { padding: var(--space-0-5); border-radius: var(--radius-xs); background: var(--fill); font-family: var(--font-mono); }
  .markdown :global(a) { color: var(--accent); text-decoration: underline; text-decoration-thickness: var(--hairline); text-underline-offset: var(--underline-offset); }
  .markdown :global(strong) { font-weight: var(--weight-semibold); }
</style>
