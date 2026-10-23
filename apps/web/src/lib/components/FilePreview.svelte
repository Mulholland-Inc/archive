<script lang="ts">
  import Icon from './Icon.svelte';
  let { src, mime, name, page }: { src: string; mime: string; name: string; page?: number } = $props();
  const kind = $derived(
    mime.startsWith('image/')
      ? 'image'
      : mime === 'application/pdf'
        ? 'pdf'
        : mime.startsWith('video/')
          ? 'video'
          : mime.startsWith('audio/')
            ? 'audio'
            : mime.startsWith('text/') || mime === 'application/json'
              ? 'text'
              : 'none'
  );
</script>

<div class="preview" data-kind={kind}>
  {#if kind === 'image'}
    <img {src} alt={name} />
  {:else if kind === 'pdf' || kind === 'text'}
    <iframe src={page ? `${src}#page=${page}` : src} title={name}></iframe>
  {:else if kind === 'video'}
    <video {src} controls preload="metadata"><track kind="captions" /></video>
  {:else if kind === 'audio'}
    <audio {src} controls preload="metadata"></audio>
  {:else}
    <div class="none"><Icon name="document" size="xl" /><span>No preview for this kind of file</span></div>
  {/if}
</div>

<style>
  .preview {
    display: grid;
    place-items: center;
    min-height: var(--space-20);
    border-radius: var(--radius-xl);
    background: var(--tint-1);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    overflow: hidden;
  }
  img,
  video {
    display: block;
    max-width: 100%;
    max-height: var(--preview-height);
    object-fit: contain;
  }
  iframe {
    width: 100%;
    height: var(--preview-height);
    border: none;
    background: var(--card);
  }
  audio {
    width: 100%;
    margin: var(--space-5);
  }
  .none {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-10);
    color: var(--muted-foreground);
  }
</style>
