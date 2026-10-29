<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { title, steps, art, alt, flip = false } = $props();
</script>

<section class="section band">
  <div class="wrap split {flip ? 'flip' : ''}">
    <div class="split-copy reveal" use:reveal>
      <h2>{#if typeof title === 'function'}{@render title()}{:else}{title}{/if}</h2>
      <ul class="letters">
        {#each steps as [name, text], i}
          <li><span class="l">{String.fromCharCode(65 + i)}</span><div><h3>{name}</h3><p>{text}</p></div></li>
        {/each}
      </ul>
    </div>
    <div class="split-art reveal" use:reveal><Art name={art} {alt} lazy /></div>
  </div>
</section>
