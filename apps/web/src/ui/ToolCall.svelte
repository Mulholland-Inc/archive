<script module>
  const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const words = /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|(`(?:\\.|[^`\\])*`|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|\b(\d[\d_.]*)\b|\b(const|let|var|await|async|return|if|else|for|of|in|new|function|true|false|null|undefined|throw|try|catch)\b/g;

  export function highlight(code) {
    let out = '';
    let at = 0;
    for (const m of code.matchAll(words)) {
      out += esc(code.slice(at, m.index));
      const kind = m[1] ? 'comment' : m[2] ? 'string' : m[3] ? 'number' : 'keyword';
      out += `<span class="${kind}">${esc(m[0])}</span>`;
      at = m.index + m[0].length;
    }
    return out + esc(code.slice(at));
  }
</script>

<script>
  let { code, result } = $props();
  const out = $derived.by(() => {
    if (result === undefined) return null;
    try {
      const r = JSON.parse(result.content);
      return { error: r.error ?? '', logs: r.logs ?? [], value: r.error ? '' : JSON.stringify(r.result, null, 2) };
    } catch {
      return { error: result.is_error ? result.content : '', logs: [], value: result.is_error ? '' : result.content };
    }
  });
</script>

<div class="call">
  <pre class="code">{@html highlight(code)}</pre>
  {#if out}
    <div class="out">
      {#if out.logs.length}<pre class="logs">{out.logs.join('\n')}</pre>{/if}
      {#if out.error}<pre class="error">{out.error}</pre>{:else}<pre>{out.value}</pre>{/if}
    </div>
  {/if}
</div>

<style>
  .call { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; }
  .out { display: flex; flex-direction: column; gap: var(--space-2); padding: var(--space-4); background: var(--raised); border-radius: var(--radius-lg); }
  pre {
    margin: 0;
    max-height: var(--tile);
    overflow: auto;
    font: var(--text-xs)/var(--leading) var(--mono);
    color: var(--text-2);
    white-space: pre;
  }
  .code { padding: var(--space-4); background: var(--panel); border-radius: var(--radius-lg); color: var(--text); }
  .code :global(.keyword) { color: var(--code-keyword); }
  .code :global(.string) { color: var(--code-string); }
  .code :global(.number) { color: var(--code-number); }
  .code :global(.comment) { color: var(--text-3); }
  .logs { color: var(--text-3); }
  .error { color: var(--alarm); white-space: pre-wrap; }
</style>
