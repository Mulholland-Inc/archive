<script>
  let { value, label, selected = false, onselect } = $props();
</script>

<button type="button" class={['card', { selected }]} role="radio" aria-checked={selected} onclick={() => onselect?.(value)}>
  <span class={['preview', value]}>
    <span class="half light"><span class="side"></span><span class="main"><span class="bar"></span><span class="bar short"></span><span class="box"></span></span></span>
    <span class="half dark"><span class="side"></span><span class="main"><span class="bar"></span><span class="bar short"></span><span class="box"></span></span></span>
  </span>
  <span class="label">{label}</span>
</button>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-1);
    border-radius: var(--radius-lg);
    text-align: center;
    color: var(--color-text-secondary);
    font-size: var(--text-sm);
    transition: color var(--duration-fast) var(--ease-out);
  }

  .card:hover,
  .selected {
    color: var(--color-text);
  }

  .preview {
    position: relative;
    display: flex;
    aspect-ratio: var(--preview-ratio);
    border-radius: var(--radius-md);
    overflow: hidden;
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border), 0 0 0 0 var(--color-accent);
    transition: box-shadow var(--duration-normal) var(--ease-out);
  }

  .selected .preview {
    box-shadow: 0 0 0 var(--focus-width) var(--color-accent);
  }

  .half {
    flex: 1;
    display: flex;
    background: var(--preview-light-bg);
  }

  .light .side { background: var(--preview-light-side); }
  .dark { background: var(--preview-dark-bg); }
  .dark .side { background: var(--preview-dark-side); }
  .light .bar { background: var(--preview-light-ink); }
  .dark .bar { background: var(--preview-dark-ink); }
  .light .box { background: var(--preview-light-surface); }
  .dark .box { background: var(--preview-dark-surface); }

  .preview.light .dark,
  .preview.dark .light {
    display: none;
  }

  .side {
    width: 28%;
  }

  .main {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1;
    padding: var(--space-2);
  }

  .bar {
    height: var(--space-1);
    width: 70%;
    border-radius: var(--radius-full);
  }

  .bar.short {
    width: 45%;
  }

  .box {
    margin-block-start: auto;
    height: 28%;
    border-radius: var(--radius-sm);
  }

  .label {
    font-weight: var(--weight-medium);
  }
</style>
