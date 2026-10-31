<script>
  import { Plus, ArrowUp, Square, Paperclip, Library, FolderPlus, Globe, Telescope, Camera, Mic } from '@lucide/svelte';
  import IconButton from './IconButton.svelte';
  import Menu from './Menu.svelte';
  import MenuItem from './MenuItem.svelte';
  import MenuSeparator from './MenuSeparator.svelte';
  import SubMenu from './SubMenu.svelte';
  import ModelPicker from './ModelPicker.svelte';
  import Attachment from './Attachment.svelte';
  import Chip from './Chip.svelte';
  import { ws } from '../lib/app.svelte.js';
  import { toast } from '../lib/toast.svelte.js';
  import { rise } from '../lib/motion.js';

  let {
    value = $bindable(''),
    placeholder = 'How can I help you today?',
    size = 'md',
    streaming = false,
    model = $bindable('pro'),
    project = $bindable(),
    onsend,
    onstop,
    element = $bindable()
  } = $props();

  let attachments = $state([]);
  let web = $state(true);
  let research = $state(false);
  let thinking = $state(true);
  let focused = $state(false);

  const control = $derived(size === 'lg' ? 'md' : 'sm');
  const ready = $derived(value.trim().length > 0 || attachments.length > 0);

  function send() {
    if (streaming) {
      onstop?.();
      return;
    }
    if (!ready) return;
    onsend?.({ text: value.trim(), attachments: [...attachments], model, research, project });
    value = '';
    attachments = [];
  }

  function onkeydown(event) {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      send();
    }
  }

  function attach(file) {
    if (attachments.some((a) => a.id === file.id)) return;
    attachments.push({ id: file.id, name: file.name, kind: file.kind, size: file.size, tone: file.tone ?? 1 });
  }

  function attachSample(kind = 'image') {
    const n = attachments.length + 1;
    const file = kind === 'screenshot'
      ? { id: `up-${Date.now()}`, name: `Screenshot ${n}.png`, kind: 'image', size: '840 KB', tone: 3 }
      : { id: `up-${Date.now()}`, name: `scan-${String(n).padStart(3, '0')}.jpg`, kind: 'image', size: '2.2 MB', tone: (n % 4) + 1 };
    attach(file);
    element?.focus();
  }

  function dictate() {
    value = (value ? value + ' ' : '') + 'What did Grandma write about the harvest in 1974?';
    element?.focus();
  }

  function pickProject(id) {
    project = project === id ? undefined : id;
    const p = ws().projects.find((x) => x.id === id);
    toast(project ? `Added to ${p?.name}` : `Removed from ${p?.name}`);
  }
</script>

<form class={['composer', size, { focused }]} onsubmit={(e) => { e.preventDefault(); send(); }}>
  {#if attachments.length}
    <div class="atts">
      {#each attachments as a (a.id)}
        <span in:rise={{ shift: '--shift-sm' }}>
          <Attachment name={a.name} kind={a.kind} size={a.size} tone={a.tone} onremove={() => (attachments = attachments.filter((x) => x.id !== a.id))} />
        </span>
      {/each}
    </div>
  {/if}
  <textarea
    bind:this={element}
    bind:value
    rows="1"
    {placeholder}
    aria-label="Message"
    {onkeydown}
    onfocus={() => (focused = true)}
    onblur={() => (focused = false)}
  ></textarea>
  <div class="bar">
    <Menu placement={size === 'lg' ? 'bottom-start' : 'top-start'} label="Add">
      {#snippet trigger(props)}
        <IconButton icon={Plus} label="Add files and more" variant="secondary" size={control} {...props} />
      {/snippet}
      <MenuItem icon={Paperclip} onselect={() => attachSample()}>Add files or photos</MenuItem>
      <MenuItem icon={Camera} onselect={() => attachSample('screenshot')}>Take a screenshot</MenuItem>
      {#if ws().library.length}
        <SubMenu icon={Library} label="Add from library">
          {#each ws().library.slice(0, 8) as file (file.id)}
            <MenuItem checked={attachments.some((a) => a.id === file.id) || undefined} onselect={() => attach(file)}>{file.name}</MenuItem>
          {/each}
        </SubMenu>
      {/if}
      {#if ws().projects.length}
        <SubMenu icon={FolderPlus} label="Add to project">
          {#each ws().projects as p (p.id)}
            <MenuItem checked={project === p.id || undefined} onselect={() => pickProject(p.id)}>{p.name}</MenuItem>
          {/each}
        </SubMenu>
      {/if}
      <MenuSeparator />
      <MenuItem icon={Telescope} keepOpen toggle={research} onselect={() => (research = !research)}>Research</MenuItem>
      <MenuItem icon={Globe} keepOpen toggle={web} onselect={() => (web = !web)}>Web search</MenuItem>
    </Menu>
    {#if research}
      <span in:rise={{ shift: '--shift-sm' }}><Chip icon={Telescope} size="sm" onremove={() => (research = false)}>Research</Chip></span>
    {/if}
    <span class="spacer"></span>
    <ModelPicker bind:model bind:thinking size={control} />
    {#if streaming}
      <IconButton icon={Square} label="Stop" variant="inverse" round size={control} type="submit" />
    {:else if size === 'lg' && !ready}
      <IconButton icon={Mic} label="Dictate" variant="ghost" round size={control} onclick={dictate} />
    {:else}
      <IconButton icon={ArrowUp} label="Send" variant="primary" round size={control} disabled={!ready} type="submit" shortcut="↵" />
    {/if}
  </div>
</form>

<style>
  .composer {
    display: flex;
    flex-direction: column;
    gap: var(--composer-gap);
    width: 100%;
    padding: var(--composer-pad);
    border-radius: var(--composer-radius);
    background: var(--color-surface);
    box-shadow: var(--shadow-composer);
    cursor: text;
    transition: box-shadow var(--duration-normal) var(--ease-out);
  }

  .focused {
    box-shadow: var(--shadow-composer-focus);
  }

  .atts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-1);
  }

  textarea {
    display: block;
    width: 100%;
    min-height: var(--composer-line);
    max-height: var(--composer-max);
    padding: var(--space-1);
    field-sizing: content;
    resize: none;
    color: var(--color-text);
    font-size: var(--composer-text);
    line-height: var(--leading-normal);
    letter-spacing: var(--tracking-normal);
  }

  .md {
    --composer-pad: var(--space-2);
    --composer-gap: var(--space-1);
    --composer-radius: var(--radius-xl);
    --composer-text: var(--text-md);
    --composer-line: var(--control-sm);
  }

  .lg {
    --composer-pad: var(--space-3);
    --composer-gap: var(--space-2);
    --composer-radius: var(--radius-2xl);
    --composer-text: var(--text-base);
    --composer-line: var(--control-2xl);
  }

  textarea::placeholder {
    color: var(--color-text-tertiary);
  }

  textarea:focus-visible {
    outline: none;
  }

  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  .spacer {
    flex: 1;
  }
</style>
