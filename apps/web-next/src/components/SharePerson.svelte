<script>
  import Avatar from './Avatar.svelte';
  import Select from './Select.svelte';
  import { permissions, isExternal, setPermission, unsharePerson } from '../lib/app.svelte.js';

  let { person, share, size = 'md', fixed } = $props();
  const options = [...permissions, { value: 'remove', label: 'Remove access' }];

  function change(value) {
    if (value === 'remove') unsharePerson(share, person.email);
    else setPermission(share, person.email, value);
  }
</script>

<div class={['person', size]}>
  <Avatar name={person.name} tone={person.tone} size={size === 'sm' ? 'sm' : 'md'} />
  <span class="who">
    <span class="name">{person.name}</span>
    <span class="meta">
      <span class="email">{person.email}</span>
      {#if !fixed && isExternal(person.email)}<span>External</span>{/if}
    </span>
  </span>
  {#if fixed}
    <span class="fixed">{fixed}</span>
  {:else}
    <Select value={person.permission} {options} size="sm" variant="ghost" placement="bottom-end" label={`Access for ${person.name}`} onchange={change} />
  {/if}
</div>

<style>
  .person {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }

  .sm {
    gap: var(--space-2);
  }

  .who {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }

  .name {
    color: var(--color-text);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
    min-width: 0;
  }

  .email {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .fixed {
    padding-inline: var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }
</style>
