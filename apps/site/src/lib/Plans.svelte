<script>
  import Button from './Button.svelte';
  import { reveal } from './reveal.js';

  let { plans } = $props();
</script>

<div class="plans reveal" use:reveal>
  {#each plans as plan}
    <div class={['plan', plan.pick && 'pick']}>
      <h3>{plan.name}</h3>
      <div class="price">{plan.price} {#if plan.per}<small>{plan.per}</small>{/if}</div>
      <p class="small">{plan.note}</p>
      <ul>
        {#each plan.items as item}<li>{item}</li>{/each}
      </ul>
      <Button href={plan.href} ghost={plan.ghost}>{plan.cta}</Button>
    </div>
  {/each}
</div>
