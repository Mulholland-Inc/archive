<script>
  import { LoaderCircle, Hand, Check, X, Square, Pause, Clock, Power, Mail } from '@lucide/svelte';
  import Badge from './Badge.svelte';
  import Icon from './Icon.svelte';

  let { status, plain = false, iconOnly = false } = $props();
  const map = {
    running: { label: 'Running', icon: LoaderCircle, spin: true },
    waiting: { label: 'Needs input', icon: Hand },
    completed: { label: 'Completed', icon: Check },
    succeeded: { label: 'Succeeded', icon: Check },
    failed: { label: 'Failed', icon: X, danger: true },
    stopped: { label: 'Stopped', icon: Square },
    paused: { label: 'Paused', icon: Pause },
    scheduled: { label: 'Scheduled', icon: Clock },
    idle: { label: 'Never run' },
    off: { label: 'Off', icon: Power },
    invited: { label: 'Invited', icon: Mail },
    connected: { label: 'Connected', icon: Check },
    active: { label: 'Active' }
  };
  const s = $derived(map[status] ?? { label: status });
</script>

{#if plain}
  <span class={['plain', { danger: s.danger, spin: s.spin }]}>
    {#if s.icon}<Icon icon={s.icon} size="sm" />{/if}
    <span class={{ 'visually-hidden': iconOnly }}>{s.label}</span>
  </span>
{:else}
  <Badge icon={s.icon} spin={s.spin} danger={s.danger}>{s.label}</Badge>
{/if}

<style>
  .plain {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1-5);
    color: var(--color-text-secondary);
    white-space: nowrap;
  }

  .plain.danger :global(.icon) {
    color: var(--color-danger);
  }

  .plain.spin :global(.icon) {
    animation: spin var(--duration-spin-status) linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(var(--turn-full));
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .plain.spin :global(.icon) {
      animation: none;
    }
  }
</style>
