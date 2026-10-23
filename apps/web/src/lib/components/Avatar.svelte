<script lang="ts">
  let {
    name,
    color = 1,
    shape = 'square',
    size = 'md',
    ring = false
  }: { name: string; color?: number; shape?: 'square' | 'circle'; size?: 'sm' | 'md' | 'lg'; ring?: boolean } = $props();
  const initials = $derived(
    name
      .split(' ')
      .filter(Boolean)
      .slice(0, 2)
      .map((p) => p[0])
      .join('')
      .toUpperCase()
  );
</script>

<span class="avatar" data-color={color} data-shape={shape} data-size={size} data-ring={ring || undefined} title={name} aria-label={name}>{initials}</span>

<style>
  .avatar {
    flex: none;
    display: inline-grid;
    place-items: center;
    width: var(--avatar-size);
    height: var(--avatar-size);
    border-radius: var(--avatar-radius);
    font-size: var(--text-2xs);
    font-weight: var(--font-medium);
    letter-spacing: var(--tracking-mono);
    line-height: 1;
    color: var(--avatar-foreground);
    user-select: none;
    transition:
      box-shadow var(--duration-fast) var(--ease-out),
      transform var(--duration-normal) var(--ease-spring);
  }
  [data-color='1'] { background: var(--avatar-1); }
  [data-color='2'] { background: var(--avatar-2); }
  [data-color='3'] { background: var(--avatar-3); }
  [data-color='4'] { background: var(--avatar-4); }
  [data-color='5'] {
    background: var(--avatar-5);
    color: var(--ink);
  }
  [data-shape='circle'] {
    border-radius: var(--radius-full);
  }
  [data-size='sm'] {
    width: var(--avatar-size-sm);
    height: var(--avatar-size-sm);
    font-size: var(--text-2xs);
  }
  [data-size='lg'] {
    width: var(--avatar-size-lg);
    height: var(--avatar-size-lg);
    font-size: var(--text-xs);
  }
  [data-ring] {
    box-shadow:
      0 0 0 var(--focus-width) var(--surface),
      0 0 0 calc(var(--focus-width) * 2) var(--ring);
  }
</style>
