<script>
  import { untrack } from 'svelte';
  import Markdown from './Markdown.svelte';

  let { text, live = false } = $props();
  let n = $state(untrack(() => (live ? 0 : text.length)));
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;

  $effect(() => {
    const end = text.length;
    if (!live || still) {
      n = end;
      return;
    }
    let frame;
    const step = () => {
      const left = end - n;
      if (left <= 0) return;
      n += Math.max(1, Math.ceil(left / 10));
      frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  });
</script>

<Markdown text={text.slice(0, n)} />
