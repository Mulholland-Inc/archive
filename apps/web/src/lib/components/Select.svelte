<script lang="ts">
  import Popover from './Popover.svelte';
  import Menu, { type MenuItem } from './Menu.svelte';
  import Button from './Button.svelte';
  let {
    icon,
    value = $bindable(),
    options,
    variant = 'tint'
  }: { icon?: string; value: string; options: MenuItem[]; variant?: 'tint' | 'secondary' | 'ghost' } = $props();
  let open = $state(false);
  const current = $derived(options.find((o) => o.id === value));
</script>

<Popover bind:open>
  {#snippet trigger({ toggle, open })}
    <Button {variant} icon={icon ?? current?.icon} trailing="chevron-down" pressed={open} onclick={toggle}>{current?.label}</Button>
  {/snippet}
  {#snippet children({ close })}
    <Menu
      items={options}
      searchable={false}
      selected={[value]}
      onselect={(item) => {
        value = item.id;
        close();
      }}
    />
  {/snippet}
</Popover>
