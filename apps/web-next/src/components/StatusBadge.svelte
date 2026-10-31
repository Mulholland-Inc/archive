<script>
  import Badge from './Badge.svelte';

  let { status } = $props();
  const map = {
    running: { tone: 'accent', label: 'Running', pulse: true },
    waiting: { tone: 'warning', label: 'Needs input' },
    completed: { tone: 'success', label: 'Completed' },
    failed: { tone: 'danger', label: 'Failed' },
    scheduled: { tone: 'neutral', label: 'Scheduled' },
    invited: { tone: 'neutral', label: 'Invited' },
    active: { tone: 'success', label: 'Active' }
  };
  const s = $derived(map[status] ?? { tone: 'neutral', label: status });
</script>

<Badge tone={s.tone} dot pulse={s.pulse}>{s.label}</Badge>
