<script>
  import Art from './Art.svelte';
  import { reveal } from './reveal.js';

  let { title, steps, art, alt, flip = false } = $props();
</script>

<section class="band">
  <div class="wrap">
    <div class={['feature', flip && 'flip']}>
      <div class="copy">
        <h2 class="reveal" use:reveal>{#if typeof title === 'function'}{@render title()}{:else}{title}{/if}</h2>
        <ul class="letters">
          {#each steps as [name, text], i}
            <li><span class="l">{String.fromCharCode(65 + i)}</span><div><h3>{name}</h3><p>{text}</p></div></li>
          {/each}
        </ul>
      </div>
      <Art class="art" reveal name={art} {alt} />
    </div>
  </div>
</section>
