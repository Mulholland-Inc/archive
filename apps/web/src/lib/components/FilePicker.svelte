<script lang="ts">
  import Button from './Button.svelte';
  let { onpick, label = 'Upload', variant = 'primary' }: { onpick: (files: File[]) => void; label?: string; variant?: 'primary' | 'secondary' } = $props();
  let input: HTMLInputElement;
</script>

<input
  bind:this={input}
  type="file"
  multiple
  hidden
  onchange={() => {
    const files = [...(input.files ?? [])];
    input.value = '';
    if (files.length) onpick(files);
  }}
/>
<Button {variant} icon="upload" onclick={() => input.click()}>{label}</Button>
