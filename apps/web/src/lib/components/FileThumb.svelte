<script lang="ts">
  import Icon from './Icon.svelte';
  let { src, mime, name, size = 'md' }: { src?: string; mime: string; name: string; size?: 'sm' | 'md' } = $props();
  let failed = $state(false);
  const icon = $derived(
    mime.startsWith('image/')
      ? 'image'
      : mime.startsWith('video/')
        ? 'film'
        : mime.startsWith('audio/')
          ? 'music'
          : mime === 'message/rfc822'
            ? 'mail'
            : 'document'
  );
</script>

<span class="thumb" data-size={size}>
  {#if src && !failed}
    <img {src} alt={name} loading="lazy" onerror={() => (failed = true)} />
  {:else}
    <Icon name={icon} size={size === 'sm' ? 'md' : 'xl'} />
  {/if}
</span>

<style>
  .thumb {
    flex: none;
    display: grid;
    place-items: center;
    width: 100%;
    aspect-ratio: 4 / 3;
    border-radius: var(--radius-md);
    background: var(--tint-1);
    color: var(--subtle-foreground);
    overflow: hidden;
  }
  [data-size='sm'] {
    width: var(--space-8);
    height: var(--space-8);
    aspect-ratio: auto;
    border-radius: var(--radius-sm);
  }
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    animation: thumb-in var(--duration-slow) var(--ease-out);
  }
  @keyframes thumb-in {
    from {
      opacity: 0;
    }
  }
</style>
