<script>
  import { HardDrive, Mail, Inbox, Calendar, Package, Cloud, Archive, Image, MessageSquare, NotebookPen, GitBranch, Plug } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { icon, size = 'md' } = $props();
  const icons = { drive: HardDrive, mail: Mail, inbox: Inbox, calendar: Calendar, package: Package, cloud: Cloud, archive: Archive, image: Image, chat: MessageSquare, notebook: NotebookPen, branch: GitBranch };
</script>

<span class={['glyph', size]}><Icon icon={icons[icon] ?? Plug} size={size === 'lg' ? 'lg' : 'md'} /></span>

<style>
  .glyph {
    display: grid;
    place-items: center;
    flex: none;
    width: var(--control-xl);
    height: var(--control-xl);
    border-radius: var(--radius-lg);
    background: var(--color-fill);
    color: var(--color-text);
  }

  .lg {
    width: var(--control-2xl);
    height: var(--control-2xl);
  }
</style>
