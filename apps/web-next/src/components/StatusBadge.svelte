<script>
  import { LoaderCircle, Hand, Check, X, Square, Pause, Clock, Power, Mail } from '@lucide/svelte';
  import Badge from './Badge.svelte';

  let { status } = $props();
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

<Badge icon={s.icon} spin={s.spin} danger={s.danger}>{s.label}</Badge>
