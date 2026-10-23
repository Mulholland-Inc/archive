<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';
  import Label from './Label.svelte';
  import Avatar from './Avatar.svelte';
  import Popover from './Popover.svelte';
  import Menu, { type MenuItem } from './Menu.svelte';

  let {
    glyph,
    title,
    labels = [],
    person,
    indent = false,
    statuses = [],
    status,
    onstatus,
    onenter,
    onleave,
    onclick,
    children
  }: {
    glyph: string;
    title: string;
    labels?: { name: string; color: number }[];
    person?: { name: string; color: number };
    indent?: boolean;
    statuses?: MenuItem[];
    status?: string;
    onstatus?: (id: string) => void;
    onenter?: (el: HTMLElement) => void;
    onleave?: () => void;
    onclick?: () => void;
    children?: Snippet;
  } = $props();

  let el: HTMLElement;
  let open = $state(false);
</script>

<div
  class="row"
  data-indent={indent || undefined}
  data-open={open || undefined}
  bind:this={el}
  role="button"
  tabindex="0"
  onpointerenter={() => onenter?.(el)}
  onpointerleave={() => onleave?.()}
  onclick={onclick}
  onkeydown={(e) => e.key === 'Enter' && onclick?.()}
>
  <span class="lead">
    <Popover bind:open>
      {#snippet trigger({ toggle })}
        <button
          class="status"
          aria-label="Change status"
          onclick={(e) => {
            e.stopPropagation();
            onleave?.();
            toggle();
          }}
        >
          {#key glyph}<span class="glyph"><Icon name={glyph} /></span>{/key}
        </button>
      {/snippet}
      {#snippet children({ close })}
        <Menu
          items={statuses}
          placeholder="Change status…"
          selected={status ? [status] : []}
          onselect={(item) => {
            onstatus?.(item.id);
            close();
          }}
        />
      {/snippet}
    </Popover>
  </span>
  <div class="content">
    <div class="line">
      <span class="title">{title}</span>
      <span class="meta">
        {#each labels as label (label.name)}<Label name={label.name} color={label.color} />{/each}
        {#if person}<Avatar name={person.name} color={person.color} size="sm" />{/if}
      </span>
    </div>
    {#if children}<div class="extra">{@render children()}</div>{/if}
  </div>
</div>

<style>
  .row {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: 0 var(--space-2) 0 var(--space-2);
    margin: 0 calc(var(--space-2) * -1);
    border-radius: var(--radius-md);
    cursor: default;
    outline: none;
    transition: background-color var(--duration-fast) var(--ease-out);
  }
  .row:hover,
  .row:focus-visible,
  .row[data-open] {
    background: var(--tint-1);
  }
  .row:focus-visible {
    box-shadow: inset 0 0 0 var(--focus-width) var(--ring);
  }
  [data-indent] {
    padding-left: calc(var(--space-2) + var(--space-9));
  }
  .lead {
    display: flex;
    align-items: center;
    height: var(--row-height);
  }
  .status {
    display: grid;
    place-items: center;
    width: calc(var(--icon-size) + var(--space-2));
    height: calc(var(--icon-size) + var(--space-2));
    margin: calc(var(--space-1) * -1);
    border-radius: var(--radius-sm);
    transition:
      background-color var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  .status:hover {
    background: var(--tint-2);
    transform: scale(var(--hover-scale));
  }
  .glyph {
    display: grid;
    animation: glyph-in var(--duration-slow) var(--ease-spring);
  }
  .content {
    flex: 1;
    min-width: 0;
    border-bottom: var(--border-width) solid var(--border);
    margin-right: calc(var(--space-2) * -1);
    padding-right: var(--space-2);
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    height: var(--row-height);
  }
  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: var(--text-md);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: var(--space-1-5);
    flex: none;
  }
  .extra {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1-5);
    margin-top: calc(var(--space-2) * -1);
    padding-bottom: var(--space-2-5);
  }
  @keyframes glyph-in {
    from {
      transform: scale(var(--enter-scale-tiny)) rotate(var(--enter-rotate));
      opacity: 0;
    }
  }
</style>
