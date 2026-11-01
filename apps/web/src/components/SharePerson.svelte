<script>
  import Avatar from './Avatar.svelte';
  import { Trash2 } from '@lucide/svelte';
  import Select from './Select.svelte';
  import IconButton from './IconButton.svelte';
  import { seedOf, permissions, isExternal, setPermission, unsharePerson } from '../lib/app.svelte.js';

  let { person, share, size = 'md' } = $props();
</script>

<div class={['person', size]}>
  <Avatar name={person.name} seed={seedOf(person)} size={size === 'sm' ? 'sm' : 'md'} />
  <span class="who">
    <span class="name">{person.name}</span>
    <span class="meta">
      <span class="email">{person.email}</span>
      {#if isExternal(person.email)}<span>External</span>{/if}
    </span>
  </span>
  <Select value={person.permission} options={permissions} size="sm" variant="ghost" placement="bottom-end" label={`Access for ${person.name}`} onchange={(v) => setPermission(share, person.email, v)} />
  <IconButton icon={Trash2} label={`Remove ${person.name}`} size="sm" onclick={() => unsharePerson(share, person.email)} />
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
</style>
