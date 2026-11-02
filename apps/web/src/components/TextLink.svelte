<script>
  // A link inside text: the text's own colour, underlined faintly until it is hovered.
  let { href, children } = $props();
</script>

<a class="link" {href}>{@render children()}</a>

<style>
  .link {
    color: var(--color-text);
    text-decoration: underline;
    text-decoration-color: var(--color-border);
    text-underline-offset: var(--underline-offset);
  }

  .link:hover {
    text-decoration-color: currentColor;
  }
</style>
