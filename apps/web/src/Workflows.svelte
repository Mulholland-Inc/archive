<script module>
  const days = { '*': 'Every day', '1-5': 'Weekdays', 'MON-FRI': 'Weekdays', '0,6': 'Weekends', '6,0': 'Weekends', 'SAT,SUN': 'Weekends',
    '0': 'Sundays', '7': 'Sundays', '1': 'Mondays', '2': 'Tuesdays', '3': 'Wednesdays', '4': 'Thursdays', '5': 'Fridays', '6': 'Saturdays',
    SUN: 'Sundays', MON: 'Mondays', TUE: 'Tuesdays', WED: 'Wednesdays', THU: 'Thursdays', FRI: 'Fridays', SAT: 'Saturdays' };
  const named = { '@hourly': ['Every hour'], '@daily': ['Every day', '0', '0'], '@midnight': ['Every day', '0', '0'], '@weekly': ['Sundays', '0', '0'],
    '@monthly': ['On the 1st of every month', '0', '0'], '@yearly': ['Every January 1st', '0', '0'], '@annually': ['Every January 1st', '0', '0'] };
  const units = { s: 'second', m: 'minute', h: 'hour' };
  const nth = (n) => n + ({ 1: 'st', 2: 'nd', 3: 'rd' }[n % 10 * (Math.floor(n / 10) % 10 !== 1)] ?? 'th');

  function schedule(spec) {
    let zone = 'UTC';
    const tz = spec.match(/^(?:CRON_)?TZ=(\S+)\s+(.*)$/);
    if (tz) [zone, spec] = [`${tz[1].split('/').pop().replaceAll('_', ' ')} time`, tz[2]];
    const at = (h, m) => ` at ${+h}:${String(+m).padStart(2, '0')} ${zone}`;
    const every = spec.match(/^@every\s+(\d+)([smh])$/);
    if (every) return +every[1] === 1 ? `Every ${units[every[2]]}` : `Every ${every[1]} ${units[every[2]]}s`;
    if (named[spec]) return named[spec][0] + (named[spec][1] ? at(named[spec][1], named[spec][2]) : '');
    const [m, h, dom, mon, dow, extra] = spec.split(/\s+/);
    const num = (s) => /^\d+$/.test(s ?? '');
    if (extra === undefined && num(m) && num(h) && mon === '*') {
      if (dom === '*' && days[dow?.toUpperCase()]) return days[dow.toUpperCase()] + at(h, m);
      if (num(dom) && dow === '*') return `On the ${nth(+dom)} of every month` + at(h, m);
    }
    if (extra === undefined && num(m) && h === '*' && dom === '*' && mon === '*' && dow === '*') return `Every hour at :${m.padStart(2, '0')}`;
    const step = m?.match(/^\*\/(\d+)$/);
    if (extra === undefined && step && h === '*' && dom === '*' && mon === '*' && dow === '*') return `Every ${step[1]} minutes`;
    return 'On a schedule';
  }

  export function when(on) {
    if (!on) return 'Manual';
    if (on === 'webhook') return 'Webhook';
    if (on.startsWith('cron ')) return schedule(on.slice(5).trim());
    const t = on.replaceAll('_', ' ');
    return `When ${/^[aeiou]/i.test(t) ? 'an' : 'a'} ${t} is added or changed`;
  }

  export const pretty = (name) => {
    const s = name.replace(/\.ts$/, '').replaceAll(/[-_]+/g, ' ');
    return s.charAt(0).toUpperCase() + s.slice(1);
  };

  const triggers = { cron: 'Schedule', table: 'Change', webhook: 'Webhook', start: 'Agent', run: 'Run button' };
  export const by = (t) => triggers[t] ?? t;

  export const stamp = (v) => new Date(v).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
</script>

<script>
  import { api } from './api.js';
  import Button from '#kit/Button.svelte';
  import Chip from './ui/Chip.svelte';
  import Copy from './ui/Copy.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import RunRow from './ui/RunRow.svelte';
  import Sheet from './ui/Sheet.svelte';

  let { name = '' } = $props();
  let flows = $state(null);
  let runs = $state(null);
  let error = $state('');
  let busy = $state('');

  const flow = $derived(flows?.find((w) => w.name === name || w.name === `${name}.ts`));
  const columns = [
    { name: 'name', type: 'text' },
    { name: 'on', label: 'When', type: 'text' },
    { name: 'last', label: 'Last run', type: 'timestamptz' },
    { name: 'run', label: '', type: 'text' },
  ];
  const path = (n) => `/api/workflows/${encodeURIComponent(n)}`;

  async function load() {
    try {
      flows = await api('/api/workflows');
      if (name) runs = await api(`${path(name)}/runs?limit=50`);
      error = '';
    } catch (e) {
      error = e.message;
    }
  }

  async function start(n, e) {
    e?.stopPropagation();
    busy = n;
    try {
      await api(`${path(n)}/run`, { method: 'POST' });
    } catch (err) {
      error = err.message;
    }
    busy = '';
    await load();
  }

  load();
</script>

{#if name}
  <Heading title={flow ? pretty(flow.name) : ''}>
    {#if flow}<Button small icon="play" onclick={() => start(flow.name)} disabled={!!busy}>{busy ? 'Running…' : 'Run'}</Button>{/if}
  </Heading>
  <Sheet>
    {#if error}<Note error>{error}</Note>{/if}
    {#if flow?.url}
      <div class="hook">
        <span class="url">{flow.url}</span>
        <Copy text={flow.url} label="Copy webhook URL" />
      </div>
    {/if}
    {#if runs?.length}
      <div class="runs">
        {#each runs as r (r.id)}
          <RunRow at={stamp(r.at)} ms={r.ms} label={by(r.trigger)} error={r.error} result={r.result} logs={r.logs} />
        {/each}
      </div>
    {:else if runs}
      <Note>No runs yet</Note>
    {/if}
  </Sheet>
{:else}
  <Heading title="Workflows" />
  <Sheet tight>
    {#if error}<Note error>{error}</Note>{/if}
    {#if flows?.length}
      <DataTable fill {columns} rows={flows} key={(r) => r.name} href={(r) => `/workflows/${encodeURIComponent(r.name.replace(/\.ts$/, ''))}`}>
        {#snippet td(r, c)}
          {#if c.name === 'name'}{pretty(r.name)}
          {:else if c.name === 'on'}{when(r.on)}
          {:else if c.name === 'last'}
            {#if r.last}<span class="last">{stamp(r.last.at)}{#if r.last.error}<Chip>Failed</Chip>{/if}</span>{/if}
          {:else}
            <span class="act"><Button small ghost onclick={(e) => start(r.name, e)} disabled={!!busy}>{busy === r.name ? 'Running…' : 'Run'}</Button></span>
          {/if}
        {/snippet}
      </DataTable>
    {:else if flows}
      <Note>No workflows yet</Note>
    {/if}
  </Sheet>
{/if}

<style>
  .hook { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2); border-radius: var(--radius-item); background: var(--fill-faint); }
  .url { flex: 1; min-width: 0; overflow: hidden; color: var(--fg-soft); font: var(--text-table) / var(--leading-medium) var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .runs { display: flex; flex-direction: column; gap: var(--space-0-5); }
  .last { display: inline-flex; align-items: center; gap: var(--space-2); }
  .act { display: flex; justify-content: flex-end; }
</style>
