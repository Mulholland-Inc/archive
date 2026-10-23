<script lang="ts">
  import Icon from './Icon.svelte';
  let {
    icon,
    field,
    operator,
    value,
    onremove,
    onvalue
  }: { icon: string; field: string; operator: string; value: string; onremove?: () => void; onvalue?: () => void } = $props();
</script>

<span class="chip">
  <span class="seg"><Icon name={icon} />{field}</span>
  <span class="seg op">{operator}</span>
  <button class="seg" onclick={onvalue}>{#key value}<span class="value">{value}</span>{/key}</button>
  <button class="seg x" aria-label={`Remove ${field} filter`} onclick={onremove}><Icon name="x" /></button>
</span>

<style>
  .chip {
    display: inline-flex;
    flex: none;
    gap: var(--space-0-5);
    height: var(--control-height);
    border-radius: var(--radius-sm);
    overflow: hidden;
    font-size: var(--text-base);
    white-space: nowrap;
    user-select: none;
  }
  .seg {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    padding: 0 var(--space-2);
    background: var(--filter-segment);
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  button.seg:hover {
    background: color-mix(in srgb, var(--filter-segment) 88%, var(--foreground));
  }
  .op {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    color: var(--muted-foreground);
  }
  .x {
    padding: 0 var(--space-1-5);
  }
  .value {
    display: inline-block;
    animation: value-in var(--duration-normal) var(--ease-out);
  }
  @keyframes value-in {
    from {
      opacity: 0;
      transform: translateY(var(--rise-offset));
    }
  }
</style>
