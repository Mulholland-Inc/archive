<script>
  // Shows wide on screens of 40rem and up and narrow below. inline leaves no box of its own around them, so
  // they sit in a toolbar's row as if they were placed there.
  let { wide, narrow, inline = false } = $props();
</script>

{#if wide}<div class={['wide', { inline }]}>{@render wide()}</div>{/if}
{#if narrow}<div class={['narrow', { inline }]}>{@render narrow()}</div>{/if}

<style>
  .narrow {
    display: none;
  }

  .inline.wide {
    display: contents;
  }

  @media (max-width: 40rem) {
    .wide,
    .inline.wide {
      display: none;
    }

    .narrow {
      display: block;
    }

    .inline.narrow {
      display: contents;
    }
  }
</style>
