<script>
  let { title, description, hero = false } = $props();
</script>

<h1 class:hero>{title}</h1>
{#if description}<p>{description}</p>{/if}

<style>
  h1 {
    font: var(--weight-medium) var(--display-lg)/1 var(--display);
    letter-spacing: var(--tracking-display);
    text-transform: capitalize;
  }
  .hero { font-size: var(--display-xl); text-transform: none; }
  p { font-size: var(--text-2xl); color: var(--text-2); margin-top: var(--space-3); max-width: var(--measure); }
</style>
