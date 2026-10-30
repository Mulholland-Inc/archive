<script>
  import robot from '../../../site/static/art/robot.webp';

  let { seed = '', size = 40 } = $props();
  const tints = ['#dbe8fb', '#f6e3dc', '#dcf1e2', '#f3ecd0', '#e8e0f6'];
  const tint = $derived(tints[[...seed].reduce((n, c) => n + c.charCodeAt(0), 0) % tints.length]);
</script>

<span class="bot" style="--tint: {tint}; --size: {size}px"><img src={robot} alt="" /></span>

<style>
  .bot { display: grid; place-items: center; flex: none; width: var(--size); height: var(--size); overflow: hidden; border-radius: 30%; background: var(--tint); }
  img { width: 62%; height: auto; }
</style>
