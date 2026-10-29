<script>
  import Actions from './Actions.svelte';
  import Scene from './Scene.svelte';

  let { title, lead, scene, alt, links, actions } = $props();
</script>

<section class="section">
  {#if scene}<div class="wrap cta-scene"><Scene name={scene} {alt} /></div>{/if}
  <div class="wrap cta">
    <h2>{title}</h2>
    <p class="lead">{lead}</p>
    <Actions {links} children={actions} />
  </div>
</section>
