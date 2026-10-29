<script>
  import Art from './Art.svelte';

  let { steps } = $props();

  let active = $state(0);

  function step(node, i) {
    const observer = new IntersectionObserver(
      (entries) => entries.forEach((e) => e.isIntersecting && (active = i)),
      { rootMargin: '-50% 0px -50% 0px' }
    );
    observer.observe(node);
    return { destroy: () => observer.disconnect() };
  }
</script>

<div class="story">
  <ol class="story-steps">
    {#each steps as s, i}
      <li class={['story-step', i === active && 'on']} use:step={i}>
        <span class="story-inline"><Art name={s.art} alt={s.alt} lazy /></span>
        <h3>{s.title}</h3>
        <p>{s.text}</p>
      </li>
    {/each}
  </ol>
  <div class="story-art" aria-hidden="true">
    {#each steps as s, i}
      <span class={['story-frame', i === active && 'on']}><Art name={s.art} lazy /></span>
    {/each}
  </div>
</div>
