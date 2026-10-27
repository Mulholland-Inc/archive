<script>
  import Icon from './Icon.svelte';

  let { title, icon, hero = false, editable = false, placeholder = 'Untitled', children, ...rest } = $props();
</script>

<div class="heading" class:hero>
  <div class="row">
    {#if icon}<span class="mark"><Icon name={icon} /></span>{/if}
    {#if editable}
      <h1 class="editable" contenteditable="plaintext-only" role="textbox" tabindex="0" data-placeholder={placeholder} {...rest}></h1>
    {:else}
      <h1>{title}</h1>
    {/if}
  </div>
  {@render children?.()}
</div>

<style>
  .heading { display: flex; flex: none; flex-direction: column; align-items: flex-start; gap: var(--space-5); }
  .row { display: flex; align-items: baseline; gap: var(--space-4); width: 100%; }
  .mark { display: flex; flex: none; }
  .mark :global(svg.icon) { width: var(--size-title-icon); height: var(--size-title-icon); }
  .hero { align-items: center; padding: 0; }
  .hero .row { justify-content: center; }
  h1 {
    min-width: 0;
    font: var(--weight-medium) var(--display-lg)/1 var(--display);
    letter-spacing: var(--tracking-display);
    text-transform: capitalize;
  }
  .hero h1 { font-size: var(--display-xl); text-transform: none; }
  .editable { flex: 1; outline: 0; text-transform: none; white-space: pre-wrap; overflow-wrap: anywhere; }
  .editable:empty::before { content: attr(data-placeholder); color: var(--text-3); pointer-events: none; }
</style>
