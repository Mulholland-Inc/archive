<script>
  import { Pencil } from '@lucide/svelte';
  import Prose from './Prose.svelte';
  import Steps from './Steps.svelte';
  import Attachment from './Attachment.svelte';
  import IconButton from './IconButton.svelte';
  import { rise } from '../lib/motion.js';

  let { message, streaming = false, onedit } = $props();
</script>

{#if message.role === 'user'}
  <article class="user" in:rise>
    {#if message.attachments?.length}
      <div class="atts">
        {#each message.attachments as a, i (a.id)}
          <Attachment name={a.name} kind={a.kind} size={a.size} tone={(i % 4) + 1} />
        {/each}
      </div>
    {/if}
    <div class="bubble">{message.text}</div>
    {#if onedit}
      <div class="actions">
        <IconButton icon={Pencil} label="Edit message" size="sm" onclick={() => onedit(message.text)} />
      </div>
    {/if}
  </article>
{:else}
  <article class="assistant" in:rise>
    {#if message.steps?.length}
      <Steps steps={message.steps} running={streaming && !message.blocks?.length} worked={message.worked} />
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
    max-width: 85%;
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

  .actions {
    display: flex;
    gap: var(--space-0-5);
    opacity: 0;
    transition: opacity var(--duration-normal) var(--ease-out);
  }

  .user:hover .actions,
  .actions:focus-within {
    opacity: 1;
  }
</style>
