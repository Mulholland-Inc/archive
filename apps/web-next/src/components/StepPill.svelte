<script>
  import { LoaderCircle, Check, ChevronUp } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { label, done = 0, total = 0, running = false, onclick } = $props();
</script>

<button type="button" class="pill" {onclick}>
  <span class={['glyph', { running }]}><Icon icon={running ? LoaderCircle : Check} size="sm" /></span>
  <span class="label">{label}</span>
  <span class="count">{done} / {total}</span>
  <Icon icon={ChevronUp} size="sm" />
</button>

<style>
  .pill {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    height: var(--control-xl);
    padding: var(--space-3);
    border-radius: var(--radius-xl);
    background: var(--color-surface);
    box-shadow: var(--ring);
    color: var(--color-text-secondary);
    font-size: var(--text-md);
    text-align: start;
    transition: box-shadow var(--duration-fast) var(--ease-out);
  }

  .pill:hover {
    box-shadow: var(--ring-strong);
  }

  .glyph {
    display: inline-flex;
    color: var(--color-text-tertiary);
  }

  .running {
    color: var(--color-text-secondary);
    animation: spin calc(var(--duration-slower) * 2) linear infinite;
  }

  .label {
    flex: 1;
    min-width: 0;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .count {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    font-variant-numeric: tabular-nums;
  }

  @keyframes spin {
    to {
      transform: rotate(1turn);
    }
  }
</style>
