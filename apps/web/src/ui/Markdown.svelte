<script module>
  const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

  function inline(s) {
    const codes = [];
    s = s.replace(/`([^`]+)`/g, (_, c) => `\u0000${codes.push(c) - 1}\u0000`);
    s = esc(s)
      .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/(^|[^*\w])\*([^*\n]+)\*(?!\w)/g, '$1<em>$2</em>')
      .replace(/(^|\W)_([^_\n]+)_(?!\w)/g, '$1<em>$2</em>');
    return s.replace(/\u0000(\d+)\u0000/g, (_, i) => `<code>${esc(codes[i])}</code>`);
  }

  const cells = (row) => row.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim());

  export function render(src) {
    const lines = src.replace(/\r/g, '').split('\n');
    const out = [];
    for (let i = 0; i < lines.length; ) {
      const line = lines[i];
      if (/^```/.test(line)) {
        const body = [];
        for (i++; i < lines.length && !/^```/.test(lines[i]); i++) body.push(lines[i]);
        i++;
        out.push(`<pre><code>${esc(body.join('\n'))}</code></pre>`);
      } else if (/^#{1,6}\s/.test(line)) {
        out.push(`<h3>${inline(line.replace(/^#+\s*/, ''))}</h3>`);
        i++;
      } else if (/^\s*\|.*\|\s*$/.test(line) && /^\s*\|?[\s:|-]+\|?\s*$/.test(lines[i + 1] ?? '') && lines[i + 1].includes('-')) {
        const head = cells(line);
        const rows = [];
        for (i += 2; i < lines.length && /^\s*\|.*\|\s*$/.test(lines[i]); i++) rows.push(cells(lines[i]));
        out.push(`<div class="table"><table><thead><tr>${head.map((c) => `<th>${inline(c)}</th>`).join('')}</tr></thead><tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${inline(c)}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`);
      } else if (/^\s*([-*+]|\d+[.)])\s/.test(line)) {
        const ordered = /^\s*\d/.test(line);
        const items = [];
        for (; i < lines.length && /^\s*([-*+]|\d+[.)])\s/.test(lines[i]); i++) items.push(lines[i].replace(/^\s*([-*+]|\d+[.)])\s+/, ''));
        const tag = ordered ? 'ol' : 'ul';
        out.push(`<${tag}>${items.map((t) => `<li>${inline(t)}</li>`).join('')}</${tag}>`);
      } else if (/^\s*(---|\*\*\*)\s*$/.test(line)) {
        out.push('<hr>');
        i++;
      } else if (!line.trim()) {
        i++;
      } else {
        const para = [];
        for (; i < lines.length && lines[i].trim() && !/^(```|#{1,6}\s|\s*([-*+]|\d+[.)])\s|\s*\|.*\|\s*$)/.test(lines[i]); i++) para.push(lines[i]);
        if (!para.length) para.push(lines[i++]);
        out.push(`<p>${para.map(inline).join('<br>')}</p>`);
      }
    }
    return out.join('');
  }
</script>

<script>
  let { text } = $props();
</script>

<div class="markdown">{@html render(text)}</div>

<style>
  .markdown { display: flex; flex-direction: column; gap: var(--space-3); min-width: 0; line-height: var(--leading-loose); overflow-wrap: anywhere; }
  .markdown :global(h3) { font-size: var(--text-lg); font-weight: var(--weight-bold); }
  .markdown :global(ul), .markdown :global(ol) { display: flex; flex-direction: column; gap: var(--space-1); margin: 0; padding: 0; list-style-position: inside; }
  .markdown :global(code) { font: var(--text-sm) var(--mono); color: var(--text-2); }
  .markdown :global(pre) { margin: 0; padding: var(--space-4); overflow-x: auto; background: var(--panel); border-radius: var(--radius-lg); }
  .markdown :global(a) { text-decoration: underline; text-decoration-color: var(--line-strong); }
  .markdown :global(hr) { width: 100%; margin: 0; border: 0; border-top: var(--stroke) solid var(--line); }
  .markdown :global(.table) { overflow-x: auto; background: var(--panel); border-radius: var(--radius-lg); }
  .markdown :global(table) { width: 100%; border-collapse: collapse; font-size: var(--text-sm); }
  .markdown :global(th), .markdown :global(td) { padding: var(--space-2); text-align: left; border-bottom: var(--stroke) solid var(--line); }
  .markdown :global(th) { color: var(--text-2); font-weight: var(--weight-medium); }
  .markdown :global(tr:last-child td) { border-bottom: 0; }
</style>
