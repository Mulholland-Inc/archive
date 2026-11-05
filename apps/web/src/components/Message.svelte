<script>
  import Prose from './Prose.svelte';
  import Steps from './Steps.svelte';
  import Attachment from './Attachment.svelte';
  import { rise } from '../lib/motion.js';
  import { contextmenu } from '../lib/contextmenu.svelte.js';
  import { messageItems } from '../lib/menus.js';
  import { openFile, ws } from '../lib/app.svelte.js';

  let { message, streaming = false } = $props();
</script>

{#if message.role === 'user'}
  <article class="user" in:rise {@attach contextmenu(() => messageItems(message))}>
    {#if message.attachments?.length}
      <div class="atts">
        {#each message.attachments as a (a.id)}
          <Attachment name={a.name} kind={a.kind} size={a.size} onopen={() => openFile(a.id, a.name)} />
        {/each}
      </div>
    {/if}
    {#if message.text}<div class="bubble">{message.text}</div>{/if}
  </article>
{:else}
  <article class="assistant" in:rise {@attach contextmenu(() => messageItems(message))}>
    {#if message.steps?.length || (streaming && !message.blocks?.length)}
      <Steps steps={message.steps ?? []} running={streaming && (!message.blocks?.length || message.steps?.at(-1)?.status === 'running')} worked={message.worked} />
    {/if}
    {#if message.blocks?.length}
      <Prose blocks={message.blocks} />
    {/if}
    {#if message.files?.length}
      <div class="files">
        {#each message.files as f (f.id)}
          <Attachment row name={ws().library.find((x) => x.id === f.id)?.name ?? f.name} kind={f.kind} size={{ created: 'Created', edited: 'Edited', read: 'Read' }[f.action]} onopen={() => openFile(f.id, f.name)} />
        {/each}
      </div>
    {/if}
  </article>
{/if}

<style>
  .user {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: var(--space-2);
  }

  .atts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--space-2);
  }

  /* Files the answer changed, framed as a table in its prose is. */
  .files {
    display: flex;
    flex-direction: column;
    max-width: var(--measure-sm);
    overflow: hidden;
    border-radius: var(--radius-lg);
    background: var(--color-surface);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .bubble {
    max-width: var(--bubble-max);
    padding: var(--space-2-5);
    border-radius: var(--radius-lg);
    background: var(--color-bubble);
    color: var(--color-text);
    font-size: var(--text-prose);
    font-family: var(--font-prose);
    line-height: var(--leading-prose);
    letter-spacing: var(--tracking-normal);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .assistant {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
</style>
