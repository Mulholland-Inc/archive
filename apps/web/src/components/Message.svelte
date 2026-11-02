<script>
  import Prose from './Prose.svelte';
  import Steps from './Steps.svelte';
  import Attachment from './Attachment.svelte';
  import { rise } from '../lib/motion.js';
  import { contextmenu } from '../lib/contextmenu.svelte.js';
  import { messageItems } from '../lib/menus.js';

  let { message, streaming = false } = $props();
</script>

{#if message.role === 'user'}
  <article class="user" in:rise {@attach contextmenu(() => messageItems(message))}>
    {#if message.attachments?.length}
      <div class="atts">
        {#each message.attachments as a, i (a.id)}
          <Attachment name={a.name} kind={a.kind} size={a.size} tone={(i % 4) + 1} />
        {/each}
      </div>
    {/if}
    <div class="bubble">{message.text}</div>
  </article>
{:else}
  <article class="assistant" in:rise {@attach contextmenu(() => messageItems(message))}>
    {#if message.steps?.length || (streaming && !message.blocks?.length)}
      <Steps steps={message.steps ?? []} running={streaming && (!message.blocks?.length || message.steps?.at(-1)?.status === 'running')} worked={message.worked} />
    {/if}
    {#if message.blocks?.length}
      <Prose blocks={message.blocks} />
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
