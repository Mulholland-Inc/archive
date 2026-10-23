<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  import Label from './Label.svelte';
  import Avatar from './Avatar.svelte';
  let {
    id,
    glyph,
    title,
    labels = [],
    priority,
    person,
    state = 'rest',
    children
  }: {
    id: string;
    glyph: string;
    title: string;
    labels?: { name: string; color: number }[];
    priority: { glyph: string; label: string };
    person?: { name: string; color: number };
    state?: 'rest' | 'lifted' | 'placeholder' | 'floating';
    children?: Snippet;
  } = $props();
</script>

<article class="card" data-state={state}>
  <div class="top">
    {#key glyph}<span class="glyph"><Icon name={glyph} /></span>{/key}
    <span class="id">{id}</span>
  </div>
  <p class="title">{title}</p>
  {#if labels.length}
    <div class="wrap">
      {#each labels as label (label.name)}<Label name={label.name} color={label.color} />{/each}
    </div>
  {/if}
  {#if children}<div class="wrap">{@render children()}</div>{/if}
  <div class="bottom">
    <span class="priority"><Icon name={priority.glyph} size="lg" />{priority.label}</span>
    {#if person}<Avatar name={person.name} color={person.color} />{:else}<Icon name="user-square" size="lg" />{/if}
  </div>
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--card-padding);
    border-radius: var(--card-radius);
    background: var(--card);
    color: var(--card-foreground);
    user-select: none;
    transition:
      box-shadow var(--duration-normal) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring),
      opacity var(--duration-normal) var(--ease-out);
  }
  .card:hover {
    box-shadow: 0 0 0 var(--border-width) var(--tint-3);
  }
  [data-state='placeholder'] {
    opacity: var(--opacity-placeholder);
    box-shadow: inset 0 0 0 var(--border-width-strong) var(--tint-3);
  }
  [data-state='lifted'] {
    transform: scale(var(--lift-scale)) rotate(var(--lift-rotate));
    box-shadow: var(--shadow-lift);
  }
  [data-state='floating'] {
    box-shadow: var(--shadow-popover);
  }
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .glyph {
    display: grid;
    animation: glyph-in var(--duration-slow) var(--ease-spring);
  }
  .id {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    letter-spacing: var(--tracking-mono);
    color: var(--subtle-foreground);
  }
  .title {
    font-size: var(--text-md);
    line-height: var(--leading-snug);
    text-wrap: pretty;
  }
  .wrap {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1-5);
  }
  .bottom {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: var(--space-1);
    color: var(--foreground);
  }
  .priority {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2-5);
    font-size: var(--text-md);
  }
  @keyframes glyph-in {
    from {
      transform: scale(var(--enter-scale-tiny)) rotate(var(--enter-rotate));
      opacity: 0;
    }
  }
</style>
