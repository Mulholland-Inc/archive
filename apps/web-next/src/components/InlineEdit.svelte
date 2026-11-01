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
    const next = node.value.replace(/\s+/g, ' ').trim();
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

{#if variant === 'heading'}
  <textarea class={['edit', variant]} rows="1" {value} aria-label={label} use:focus {onkeydown} onblur={(event) => finish(true, event.currentTarget)}></textarea>
{:else}
  <input class={['edit', variant]} type="text" {value} aria-label={label} use:focus {onkeydown} onblur={(event) => finish(true, event.currentTarget)} />
{/if}

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

  .heading {
    display: block;
    width: calc(100% + var(--space-3) * 2);
    max-width: none;
    height: auto;
    margin: calc(var(--space-1) * -1) calc(var(--space-3) * -1);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-md);
    field-sizing: content;
    resize: none;
    overflow: hidden;
    line-height: inherit;
    letter-spacing: inherit;
  }
</style>
