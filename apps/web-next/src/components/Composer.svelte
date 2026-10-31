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
  import { library, projects } from '../lib/data.js';
  import { rise } from '../lib/motion.js';

  let {
    value = $bindable(''),
    placeholder = 'How can I help you today?',
    variant = 'chat',
    streaming = false,
    model = $bindable('pro'),
    onsend,
    onstop,
    element = $bindable()
  } = $props();

  let attachments = $state([]);
  let web = $state(true);
  let research = $state(false);
  let thinking = $state(true);
  let focused = $state(false);

  const ready = $derived(value.trim().length > 0 || attachments.length > 0);

  function send() {
    if (streaming) {
      onstop?.();
      return;
    }
    if (!ready) return;
    onsend?.({ text: value.trim(), attachments: [...attachments], model, research });
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

  function attachSample() {
    attach({ id: `up-${Date.now()}`, name: 'scan-' + (attachments.length + 1) + '.jpg', kind: 'image', size: '2.2 MB', tone: (attachments.length % 4) + 1 });
  }
</script>

<form class={['composer', variant, { focused }]} onsubmit={(e) => { e.preventDefault(); send(); }}>
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
    <Menu placement={variant === 'home' ? 'bottom-start' : 'top-start'} label="Add">
      {#snippet trigger(props)}
        <IconButton icon={Plus} label="Add files and more" variant="secondary" {...props} />
      {/snippet}
      <MenuItem icon={Paperclip} onselect={attachSample}>Add files or photos</MenuItem>
      <MenuItem icon={Camera} onselect={attachSample}>Take a screenshot</MenuItem>
      <SubMenu icon={Library} label="Add from library">
        {#each library.slice(0, 6) as file (file.id)}
          <MenuItem onselect={() => attach(file)}>{file.name}</MenuItem>
        {/each}
      </SubMenu>
      <SubMenu icon={FolderPlus} label="Add to project">
        {#each projects.slice(0, 5) as p (p.id)}
          <MenuItem>{p.name}</MenuItem>
        {/each}
      </SubMenu>
      <MenuSeparator />
      <MenuItem icon={Telescope} keepOpen toggle={research} onselect={() => (research = !research)}>Research</MenuItem>
      <MenuItem icon={Globe} keepOpen toggle={web} onselect={() => (web = !web)}>Web search</MenuItem>
    </Menu>
    {#if research}
      <span in:rise={{ shift: '--shift-sm' }}><Chip icon={Telescope} size="sm" onremove={() => (research = false)}>Research</Chip></span>
    {/if}
    <span class="spacer"></span>
    <ModelPicker bind:model bind:thinking />
    {#if streaming}
      <IconButton icon={Square} label="Stop" variant="inverse" round type="submit" />
    {:else if variant === 'home' && !ready}
      <IconButton icon={Mic} label="Dictate" variant="ghost" round />
    {:else}
      <IconButton icon={ArrowUp} label="Send" variant="primary" round disabled={!ready} type="submit" shortcut="↵" />
    {/if}
  </div>
</form>

<style>
  .composer {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-3);
    border-radius: var(--radius-2xl);
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
    min-height: var(--control-lg);
    max-height: var(--composer-max);
    padding: var(--space-1);
    field-sizing: content;
    resize: none;
    color: var(--color-text);
    font-size: var(--text-base);
    line-height: var(--leading-normal);
    letter-spacing: var(--tracking-normal);
  }

  .home textarea {
    min-height: var(--control-2xl);
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
