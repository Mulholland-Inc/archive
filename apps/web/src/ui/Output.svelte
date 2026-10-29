<script>
  import { highlight } from './CodeBlock.svelte';

  let { text, tone = '' } = $props();
</script>

<pre class={tone}>{#if tone}{text}{:else}<code>{@html highlight(text)}</code>{/if}</pre>

<style>
  pre {
    max-height: var(--output-height);
    padding: var(--space-4);
    overflow: auto;
    border-radius: var(--radius-item);
    background: var(--fill-faint);
    color: var(--fg);
    font: var(--text-chat) / var(--leading-ui) var(--font-mono);
    white-space: pre;
  }
  .muted { color: var(--fg-muted); }
  .danger { color: var(--danger); white-space: pre-wrap; overflow-wrap: anywhere; }
  pre :global(.keyword) { color: var(--syntax-keyword); }
  pre :global(.string) { color: var(--syntax-string); }
  pre :global(.number) { color: var(--syntax-number); }
</style>
