<script>
  import { Mail, X } from '@lucide/svelte';
  import Avatar from './Avatar.svelte';
  import Icon from './Icon.svelte';
  import { uid } from '../lib/ids.js';
  import { show, hide } from '../lib/floating.js';
  import { seedOf } from '../lib/app.svelte.js';

  let { value = $bindable([]), people = [], placeholder = 'Add people by name or email', label = 'People', limit = 6 } = $props();

  const listId = uid('people');
  let box = $state();
  let panel = $state();
  let input = $state();
  let query = $state('');
  let active = $state(0);
  let opened = $state(false);

  const isEmail = (s) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(s);
  const q = $derived(query.trim().toLowerCase());
  const taken = $derived(new Set(value.map((p) => p.email)));
  const matches = $derived(people.filter((p) => !taken.has(p.email) && (!q || p.name.toLowerCase().includes(q) || p.email.toLowerCase().includes(q))).slice(0, limit));
  const fresh = $derived(isEmail(q) && !taken.has(q) && !people.some((p) => p.email.toLowerCase() === q) ? { name: q, email: q, fresh: true } : null);
  const options = $derived(fresh ? [fresh, ...matches] : matches);
  const shown = $derived(opened && (options.length > 0 || q.length > 0));

  $effect(() => {
    if (!panel) return;
    if (shown) show(panel, box);
    else hide(panel);
  });

  $effect(() => {
    q;
    active = 0;
  });

  $effect(() => {
    if (shown) document.getElementById(`${listId}-${active}`)?.scrollIntoView({ block: 'nearest' });
  });

  function pick(person) {
    if (!person || taken.has(person.email)) return;
    const { fresh: _, ...rest } = person;
    value = [...value, rest];
    query = '';
    opened = false;
    input?.focus();
  }

  function remove(email) {
    value = value.filter((p) => p.email !== email);
    input?.focus();
  }

  function byEmail(email) {
    const e = email.toLowerCase();
    return people.find((p) => p.email.toLowerCase() === e) ?? { name: e, email: e };
  }

  function keydown(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      if (!options.length) return;
      event.preventDefault();
      if (!shown) return void (opened = true);
      active = (active + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length;
    } else if (event.key === 'Enter' && shown && options[active]) {
      event.preventDefault();
      pick(options[active]);
    } else if ((event.key === ',' || event.key === ';' || event.key === ' ') && isEmail(q)) {
      event.preventDefault();
      pick(byEmail(q));
    } else if (event.key === 'Backspace' && !query && value.length) {
      value = value.slice(0, -1);
    } else if (event.key === 'Escape' && shown) {
      event.preventDefault();
      event.stopPropagation();
      query = '';
      opened = false;
    }
  }

  function paste(event) {
    const found = (event.clipboardData?.getData('text') ?? '').split(/[\s,;]+/).filter(isEmail);
    if (found.length < 2) return;
    event.preventDefault();
    const add = [...new Set(found.map((e) => e.toLowerCase()))].filter((e) => !taken.has(e)).map(byEmail);
    value = [...value, ...add];
  }

  function parts(text) {
    if (!q) return [{ text, hit: false }];
    const i = text.toLowerCase().indexOf(q);
    if (i < 0) return [{ text, hit: false }];
    return [
      { text: text.slice(0, i), hit: false },
      { text: text.slice(i, i + q.length), hit: true },
      { text: text.slice(i + q.length), hit: false }
    ].filter((p) => p.text);
  }
</script>

{#snippet highlight(text)}{#each parts(text) as part, i (i)}{#if part.hit}<mark>{part.text}</mark>{:else}{part.text}{/if}{/each}{/snippet}

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div bind:this={box} class={['box', { open: shown }]} onclick={() => { input?.focus(); opened = true; }}>
  {#each value as person (person.email)}
    <span class="token">
      <Avatar name={person.name} seed={seedOf(person)} size="xs" />
      <span class="token-name">{person.name}</span>
      <button type="button" class="token-remove" aria-label={`Remove ${person.name}`} onclick={(e) => { e.stopPropagation(); remove(person.email); }}>
        <Icon icon={X} size="xs" />
      </button>
    </span>
  {/each}
  <input
    bind:this={input}
    bind:value={query}
    type="text"
    role="combobox"
    autocomplete="off"
    aria-label={label}
    aria-autocomplete="list"
    aria-expanded={shown}
    aria-controls={listId}
    aria-activedescendant={shown && options[active] ? `${listId}-${active}` : undefined}
    placeholder={value.length ? '' : placeholder}
    onblur={() => (opened = false)}
    oninput={() => (opened = true)}
    onkeydown={keydown}
    onpaste={paste}
  />
</div>

<div bind:this={panel} id={listId} popover="manual" role="listbox" aria-label={label} class="list floating" data-placement="bottom-start" data-match>
  {#each options as person, i (person.email)}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
      id={`${listId}-${i}`}
      role="option"
      tabindex="-1"
      aria-selected={i === active}
      class={['option', { active: i === active }]}
      onmousedown={(e) => e.preventDefault()}
      onmousemove={() => (active = i)}
      onclick={() => pick(person)}
    >
      {#if person.fresh}
        <span class="glyph"><Icon icon={Mail} size="md" /></span>
        <span class="who">
          <span class="name">Invite {person.email}</span>
          <span class="meta">Not in this archive yet</span>
        </span>
      {:else}
        <Avatar name={person.name} seed={seedOf(person)} size="md" />
        <span class="who">
          <span class="name">{@render highlight(person.name)}</span>
          <span class="meta">{@render highlight(person.email)}</span>
        </span>
        {#if person.guest}<span class="tag">Guest</span>{/if}
      {/if}
    </div>
  {:else}
    <p class="empty">No one matches. Type a full email to invite someone new.</p>
  {/each}
</div>

<style>
  .box {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-1-5);
    min-height: var(--control-xl);
    padding: var(--space-1-5) var(--space-2);
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
    cursor: text;
    transition:
      box-shadow var(--duration-fast) var(--ease-out),
      background-color var(--duration-fast) var(--ease-out);
    min-width: 0;
  }

  .box:hover {
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-strong);
  }

  .box:focus-within {
    box-shadow:
      inset 0 0 0 var(--border-width) var(--color-accent),
      0 0 0 calc(var(--focus-width) + var(--border-width)) var(--color-accent-subtle);
  }

  input {
    flex: 1 1 var(--space-16);
    min-width: var(--space-16);
    height: var(--control-xs);
    padding-inline: var(--space-1);
    color: var(--color-text);
  }

  input::placeholder {
    color: var(--color-text-tertiary);
  }

  input:focus-visible {
    outline: none;
  }

  .token {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    max-width: 100%;
    height: var(--control-sm);
    padding-inline: var(--space-1) var(--space-0-5);
    border-radius: var(--radius-full);
    background: var(--color-fill);
    color: var(--color-text);
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    animation: token-in var(--duration-fast) var(--ease-out);
  }

  .token-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .token-remove {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-xs);
    height: var(--control-xs);
    border-radius: var(--radius-full);
    color: var(--color-text-tertiary);
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .token-remove:hover {
    background: var(--color-fill-strong);
    color: var(--color-text);
  }

  @keyframes token-in {
    from {
      opacity: 0;
      transform: scale(var(--scale-enter));
    }
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-px);
    max-height: var(--picker-max-height);
    overflow-y: auto;
    padding: var(--space-1-5);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    box-shadow: var(--ring);
  }

  .option {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2);
    border-radius: var(--radius-md);
    cursor: pointer;
  }

  .active {
    background: var(--color-fill);
  }

  .glyph {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--avatar-md);
    height: var(--avatar-md);
    border-radius: var(--radius-full);
    background: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  .who {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }

  .name,
  .meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .name {
    color: var(--color-text);
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  .meta {
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  mark {
    background: none;
    color: var(--color-accent);
  }

  .tag {
    flex: none;
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }

  .empty {
    padding: var(--space-3) var(--space-2);
    color: var(--color-text-tertiary);
    font-size: var(--text-sm);
  }
</style>
