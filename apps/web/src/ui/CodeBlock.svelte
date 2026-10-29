<script module>
  const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const words = /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|(`(?:\\.|[^`\\])*`?|"(?:\\.|[^"\\\n])*"?|'(?:\\.|[^'\\\n])*'?)|\b(\d[\d_.]*)\b|\b(const|let|var|await|async|return|if|else|for|of|in|new|function|true|false|null|undefined|throw|try|catch|import|export|from|type|interface|SELECT|FROM|WHERE|AND|OR|ORDER|BY|GROUP|LIMIT|JOIN|ON|AS|COUNT|SUM|INSERT|UPDATE|DELETE|INTO|VALUES|SET)\b/g;

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
  import Copy from './Copy.svelte';

  let { code, copy = true, live = false } = $props();
</script>

<div class="block">
  <pre class:live><code>{@html highlight(code)}</code></pre>
  {#if copy && code}<span class="tool"><Copy text={code} label="Copy code" /></span>{/if}
</div>

<style>
  .block { position: relative; min-width: 0; }
  pre {
    padding: var(--space-4);
    overflow-x: auto;
    border-radius: var(--radius-item);
    background: var(--fill-faint);
    color: var(--fg);
    font: var(--text-chat) / var(--leading-ui) var(--font-mono);
    white-space: pre;
  }
  .live code::after { content: '▍'; color: var(--fg-muted); animation: blink var(--duration-loop) steps(1) infinite; }
  .tool { position: absolute; top: var(--space-1); right: var(--space-1); opacity: 0; transition: opacity var(--duration-fast); }
  .block:hover .tool, .tool:focus-within { opacity: 1; }
  pre :global(.keyword) { color: var(--syntax-keyword); }
  pre :global(.string) { color: var(--syntax-string); }
  pre :global(.number) { color: var(--syntax-number); }
  pre :global(.comment) { color: var(--syntax-comment); }
  @keyframes blink { 50% { opacity: 0; } }
  @media (hover: none) { .tool { opacity: 1; } }
  @media (prefers-reduced-motion: reduce) { .live code::after { animation: none; } }
</style>
