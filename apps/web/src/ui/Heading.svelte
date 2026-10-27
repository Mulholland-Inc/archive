<script>
  let { title, hero = false, children } = $props();
</script>

<div class:hero>
  <h1>{title}</h1>
  {@render children?.()}
</div>

<style>
  div { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-5); padding: var(--inset); }
  .hero { align-items: center; padding: 0; }
  h1 {
    font: var(--weight-medium) var(--display-lg)/1 var(--display);
    letter-spacing: var(--tracking-display);
    text-transform: capitalize;
  }
  .hero h1 { font-size: var(--display-xl); text-transform: none; }
</style>
