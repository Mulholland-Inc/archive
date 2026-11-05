<script>
  import Dialog from './Dialog.svelte';
  import Field from './Field.svelte';
  import Input from './Input.svelte';
  import Textarea from './Textarea.svelte';
  import { Paintbrush } from '@lucide/svelte';
  import Button from './Button.svelte';
  import IconButton from './IconButton.svelte';
  import LookPicker from './LookPicker.svelte';
  import { validateAvatar } from 'open-bot-avatar';
  import { run } from '../lib/api.js';
  import { TINTS } from '../lib/looks.js';
  import { AVATARS } from '../lib/avatars.js';
  import { app, agentById, createAgent, updateAgent } from '../lib/app.svelte.js';

  // app.agentDialog is the id of the agent being changed, true for a new one, or null.
  const agent = $derived(agentById(app.agentDialog));
  const open = $derived(Boolean(app.agentDialog));

  // One drawn here or by the agent itself is a choice too.
  let drawn = $state([]);
  let brief = $state('');
  let drawing = $state(false);
  let failed = $state('');

  const choices = $derived([...drawn, ...(agent?.avatar && !AVATARS.some((a) => a.id === agent.avatar.id) ? [agent.avatar] : []), ...AVATARS]);

  let name = $state('');
  let look = $state({ avatar: AVATARS[0], tint: 0 });
  let instructions = $state('');

  $effect(() => {
    if (!open) return;
    drawn = [];
    brief = '';
    failed = '';
    name = agent?.name ?? '';
    instructions = agent?.instructions ?? '';
    look = agent ? { avatar: agent.avatar ?? AVATARS.find((a) => a.id === 'chat'), tint: agent.tint } : { avatar: AVATARS[Math.floor(Math.random() * AVATARS.length)], tint: Math.floor(Math.random() * TINTS) };
  });

  // Asks a model for an avatar from the words typed here, with the library's own design prompt, and once more
  // with the reasons if the renderer refuses what it drew.
  async function draw() {
    drawing = true;
    failed = '';
    try {
      const system = (await import('../lib/avatar-prompt.md?raw')).default;
      let prompt = brief.trim();
      for (let tries = 0; ; tries++) {
        const { text } = await run(`return await archive.model(${JSON.stringify({ model: 'claude-haiku-5-5', system, prompt })})`);
        let checked;
        try {
          checked = validateAvatar(JSON.parse(text.replace(/^[^{]*|[^}]*$/g, '')));
        } catch {
          checked = { ok: false, errors: ['the answer was not JSON'] };
        }
        if (checked.ok) {
          const avatar = { ...checked.value, id: `${checked.value.id}-${crypto.randomUUID().slice(0, 4)}` };
          drawn = [avatar, ...drawn];
          look = { ...look, avatar };
          brief = '';
          return;
        }
        if (tries === 1) throw new Error(`Couldn’t draw that: ${checked.errors[0]}`);
        prompt = `${brief.trim()}\n\nThe engine refused your last answer: ${checked.errors.join('; ')}\n\n${text}\n\nFix it.`;
      }
    } catch (e) {
      failed = e.message;
    } finally {
      drawing = false;
    }
  }

  function save(close) {
    const row = { name: name.trim(), avatar: look.avatar, tint: look.tint, instructions };
    if (agent) updateAgent(agent.id, row);
    else createAgent(row);
    close();
  }
</script>

<Dialog bind:open={() => open, (on) => { if (!on) app.agentDialog = null; }} title={agent ? 'Agent settings' : 'New agent'} fit>
  <Field label="Name">
    {#snippet children(id)}
      <Input {id} bind:value={name} />
    {/snippet}
  </Field>
  <LookPicker bind:look avatars={choices} />
  <Field error={failed}>
    {#snippet children(id)}
      <div class="draw">
        <Input {id} bind:value={brief} placeholder="Describe an avatar" onkeydown={(e) => { if (e.key === 'Enter' && brief.trim() && !drawing) { e.preventDefault(); draw(); } }} />
        <span class="brush"><IconButton icon={Paintbrush} label={drawing ? 'Drawing' : 'Draw'} size="sm" disabled={!brief.trim() || drawing} onclick={draw} /></span>
      </div>
    {/snippet}
  </Field>
  <div class="grow">
    <Field label="Instructions">
      {#snippet children(id)}
        <Textarea {id} bind:value={instructions} rows={3} autosize />
      {/snippet}
    </Field>
  </div>
  {#snippet footer({ close })}
    <Button variant="ghost" onclick={close}>Cancel</Button>
    <Button variant="primary" disabled={!name.trim()} onclick={() => save(close)}>{agent ? 'Save' : 'Create agent'}</Button>
  {/snippet}
</Dialog>

<style>
  .draw {
    position: relative;
  }

  .draw :global(input) {
    width: 100%;
    padding-inline-end: var(--control-md);
  }

  .brush {
    position: absolute;
    inset-block: 0;
    inset-inline-end: var(--space-1);
    display: flex;
    align-items: center;
  }

  /* The instructions take what height the popup has left, and past that scroll inside. */
  .grow {
    display: flex;
    flex-direction: column;
    flex: 0 1 auto;
    min-height: 0;
  }

  .grow :global(.field) {
    flex: 0 1 auto;
    min-height: 0;
  }

  .grow :global(textarea) {
    flex: 0 1 auto;
    overflow-y: auto;
  }
</style>
