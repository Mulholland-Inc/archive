<script lang="ts">
  import { Dialog, Field, Input, Button, Notice, Select, Stack } from '../../lib';
  import { api } from '../../lib/api';
  import { session, reloadViews, spaceLabel } from '../../lib/session.svelte';
  import { humanize } from '../../lib/spec';
  import { navigate } from '../../lib/router.svelte';
  import { toast } from '../../lib/toast.svelte';

  let {
    open = $bindable(false),
    type,
    layout,
    by,
    where
  }: { open?: boolean; type: string; layout: string; by?: string; where: { [k: string]: string } } = $props();

  let name = $state('');
  let space = $state('');
  let busy = $state(false);
  let failure = $state('');
  const writable = $derived(session.spaces.filter((s) => ['editor', 'admin', 'owner'].includes(s.level)));

  $effect(() => {
    if (open) {
      name = '';
      failure = '';
      space = (writable.find((s) => s.name === session.space) ?? writable.find((s) => s.id === session.home) ?? writable[0])?.name ?? '';
    }
  });

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    if (!name.trim()) {
      failure = 'Give the view a name.';
      return;
    }
    busy = true;
    failure = '';
    try {
      const v = await api.addView({ name: name.trim(), type, layout, by: by || undefined, where, space: space || undefined });
      await reloadViews();
      open = false;
      toast(`Saved ${v.name}`, 'success');
      navigate(`/views/${v.id}`);
    } catch (err) {
      failure = (err as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog bind:open title="Save view" description="Keep these filters and this layout in the sidebar, shared with everyone in the space.">
  <form onsubmit={submit} novalidate>
    <Stack gap="4">
    {#if failure}<Notice tone="danger">{failure}</Notice>{/if}
    <Field label="Name">
      <Input bind:value={name} placeholder={`Unpaid ${humanize(type).toLowerCase()}s`} autofocus label="View name" />
    </Field>
    {#if writable.length > 1}
      <Field label="Space">
        <Select variant="field" bind:value={space} options={writable.map((s) => ({ id: s.name, label: spaceLabel(s.name), icon: 'folder' }))} label="Space" />
      </Field>
    {/if}
    </Stack>
  </form>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (open = false)}>Cancel</Button>
    <Button variant="primary" loading={busy} onclick={() => submit()}>Save view</Button>
  {/snippet}
</Dialog>
