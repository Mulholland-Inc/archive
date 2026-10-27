<script>
  let { title, description, hero = false } = $props();
</script>

<div class:hero>
  <h1>{title}</h1>
  {#if description}<p>{description}</p>{/if}
</div>

<style>
  div { padding: 0 var(--inset); }
  .hero { padding: 0; }
  h1 {
    font: var(--weight-medium) var(--display-lg)/1 var(--display);
    letter-spacing: var(--tracking-display);
    text-transform: capitalize;
  }
  .hero h1 { font-size: var(--display-xl); text-transform: none; }
  p { font-size: var(--text-2xl); color: var(--text-2); margin-top: var(--space-3); max-width: var(--measure); }
</style>
