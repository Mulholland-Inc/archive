<script>
  let { value, label, variant = 'row', onsave, oncancel } = $props();

  let done = false;

  function focus(node) {
    requestAnimationFrame(() => {
      node.focus();
      node.select();
    });
  }

  function finish(save, node) {
    if (done) return;
    done = true;
    const next = node.value.trim();
    if (save && next && next !== value) onsave(next);
    else oncancel();
  }

  function onkeydown(event) {
    if (event.key === 'Enter') {
      event.preventDefault();
      finish(true, event.currentTarget);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      finish(false, event.currentTarget);
    }
  }
</script>

<input class={['edit', variant]} type="text" {value} aria-label={label} use:focus {onkeydown} onblur={(event) => finish(true, event.currentTarget)} />

<style>
  .edit {
    min-width: 0;
    height: var(--control-md);
    border: none;
    outline: none;
    background: transparent;
    box-shadow: inset var(--ring-strong);
    color: var(--color-text);
    font: inherit;
  }

  .row {
    font-size: var(--text-md);
    flex: 1;
    padding: var(--space-1-5);
    border-radius: inherit;
  }

  .title {
    font-size: var(--text-md);
    field-sizing: content;
    max-width: 100%;
    padding-inline: var(--space-3);
    border-radius: var(--radius-md);
    font-weight: var(--weight-medium);
    line-height: var(--leading-none);
  }

  .plain {
    field-sizing: content;
    max-width: 100%;
    height: auto;
    padding: 0;
    border-radius: var(--radius-xs);
    box-shadow: none;
    background: var(--color-fill);
  }
</style>
