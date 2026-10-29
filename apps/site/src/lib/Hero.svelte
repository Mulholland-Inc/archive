<script>
  import Actions from './Actions.svelte';
  import Art from './Art.svelte';

  import Icon from './Icon.svelte';

  let { kind = 'page', title, lead, art, alt, links, actions, badges = [], children } = $props();
</script>

{#if kind === 'home'}
  <section class="home-hero">
    <div class="wrap home-copy">
      <h1>{title}</h1>
      <p class="lead">{lead}</p>
      <Actions {links} children={actions} />
      {#if badges.length}<ul class="badges">{#each badges as b}<li><Icon name="shield" />{b}</li>{/each}</ul>{/if}
    </div>
  </section>
{:else if kind === 'post'}
  <section class="post-hero">
    <h1>{title}</h1>
    <p class="lead">{lead}</p>
  </section>
  <div class="post-art"><Art name={art} {alt} /></div>
{:else if kind === 'blog'}
  <section class="blog-head">
    <div class="wrap">
      <h1>{title}</h1>
      <p>{lead}</p>
    </div>
  </section>
{:else}
  <section class="page-hero">
    <div class="wrap">
      <div class="page-copy">
        <h1>{title}</h1>
        <p class="lead">{lead}</p>
        <Actions {links} children={actions} />
        {@render children?.()}
      </div>
      <div class="page-art"><Art name={art} {alt} /></div>
    </div>
  </section>
{/if}
