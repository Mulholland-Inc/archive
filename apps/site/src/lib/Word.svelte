<script>
  import { WORD } from './site.js';

  let { class: cls = 'word' } = $props();
</script>

<svg class={cls} viewBox="417 135 1662 403" aria-hidden="true">{#each WORD as d}<path {d} />{/each}</svg>
