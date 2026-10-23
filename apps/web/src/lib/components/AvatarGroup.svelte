<script lang="ts">
  import Avatar from './Avatar.svelte';
  let {
    people,
    selected = [],
    ontoggle
  }: {
    people: { name: string; color: number }[];
    selected?: string[];
    ontoggle?: (name: string) => void;
  } = $props();
</script>

<div class="group" role="group" aria-label="People">
  {#each people as person (person.name)}
    <button class="member" aria-pressed={selected.includes(person.name)} onclick={() => ontoggle?.(person.name)}>
      <Avatar name={person.name} color={person.color} ring={selected.includes(person.name)} />
    </button>
  {/each}
</div>

<style>
  .group {
    display: inline-flex;
    align-items: center;
  }
  .member {
    display: grid;
    border-radius: var(--avatar-radius);
    margin-left: calc(var(--space-0-5) * -1);
    box-shadow: 0 0 0 var(--border-width-strong) var(--surface);
    transition: transform var(--duration-normal) var(--ease-spring);
  }
  .member:first-child {
    margin-left: 0;
  }
  .member:hover {
    transform: translateY(calc(var(--space-0-5) * -1));
    z-index: var(--z-raised);
  }
  .member:active {
    transform: scale(var(--press-scale));
  }
</style>
