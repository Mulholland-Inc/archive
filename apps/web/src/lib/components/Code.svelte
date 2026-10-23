<script lang="ts">
  import IconButton from './IconButton.svelte';
  import { toast } from '../toast.svelte';
  let { value, copy = true, block = false }: { value: string; copy?: boolean; block?: boolean } = $props();
  async function doCopy() {
    try {
      await navigator.clipboard.writeText(value);
      toast('Copied to the clipboard', 'success');
    } catch {
      toast('Could not copy; select the text and copy it yourself', 'danger');
    }
  }
</script>

<div class="code" data-block={block || undefined}>
  <code>{value}</code>
  {#if copy}<IconButton icon="copy" label="Copy" size="sm" onclick={doCopy} />{/if}
</div>

<style>
  .code {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    padding: var(--space-1) var(--space-1) var(--space-1) var(--space-3);
    border-radius: var(--radius-md);
    background: var(--tint-1);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
  }
  code {
    flex: 1;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    overflow-wrap: anywhere;
    user-select: all;
  }
  [data-block] {
    align-items: flex-start;
    padding: var(--space-3);
  }
  [data-block] code {
    white-space: pre-wrap;
    max-height: var(--preview-height);
    overflow: auto;
    user-select: text;
  }
</style>
