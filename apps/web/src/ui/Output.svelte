<script>
  import Button from '#kit/Button.svelte';
  import { highlight } from './CodeBlock.svelte';

  let { text, tone = '' } = $props();
  let all = $state(false);
  let box = $state();
  let long = $state(false);

  $effect(() => {
    text;
    long = box.scrollHeight > box.clientHeight || all;
  });
</script>

<div class="output">
  <pre bind:this={box} class={tone} class:all>{#if tone}{text}{:else}<code>{@html highlight(text)}</code>{/if}</pre>
  {#if long}<div class="more"><Button ghost small onclick={() => (all = !all)}>{all ? 'Show less' : 'Show all'}</Button></div>{/if}
</div>

<style>
  .output { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; }
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
  .all { max-height: none; }
  .muted { color: var(--fg-muted); }
  .danger { color: var(--danger); white-space: pre-wrap; overflow-wrap: anywhere; }
  .more { display: flex; justify-content: center; }
  pre :global(.keyword) { color: var(--syntax-keyword); }
  pre :global(.string) { color: var(--syntax-string); }
  pre :global(.number) { color: var(--syntax-number); }
</style>
