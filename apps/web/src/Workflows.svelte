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

  export const stamp = (v) => new Date(v).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
</script>

<script>
  import { api } from './api.js';
  import Icon from '#kit/Icon.svelte';
  import Chip from './ui/Chip.svelte';
  import DataTable from './ui/DataTable.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';
  import Empty from './ui/Empty.svelte';
  import Sheet from './ui/Sheet.svelte';

  let flows = $state(null);
  let error = $state('');
  let busy = $state('');

  const columns = [
    { name: 'name', type: 'text' },
    { name: 'on', label: 'When', type: 'text' },
    { name: 'last', label: 'Last run', type: 'timestamptz' },
    { name: 'act', label: '', type: 'text' },
  ];
  const path = (n) => `/api/workflows/${encodeURIComponent(n)}`;

  async function load() {
    try {
      flows = await api('/api/workflows');
      error = '';
    } catch (e) {
      error = e.message;
    }
  }

  async function act(n, method, url) {
    busy = n;
    try {
      await api(url, { method });
    } catch (e) {
      error = e.message;
    }
    busy = '';
    await load();
  }

  load();
</script>

<Heading title="Workflows" />
<Sheet tight>
  {#if error}<Note error>{error}</Note>{/if}
  {#if flows?.length}
    <DataTable fill {columns} rows={flows} key={(r) => r.name}>
      {#snippet td(r, c)}
        {#if c.name === 'name'}{pretty(r.name)}
        {:else if c.name === 'on'}{when(r.on)}
        {:else if c.name === 'last'}
          {#if r.last}<span class="last" title={r.last.error || undefined}>{stamp(r.last.at)}{#if r.last.error}<Chip>Failed</Chip>{/if}</span>{/if}
        {:else}
          <span class="acts">
            <button type="button" title="Run" aria-label="Run {pretty(r.name)}" disabled={!!busy} onclick={() => act(r.name, 'POST', `${path(r.name)}/run`)}><Icon name="play" /></button>
            <button type="button" title="Delete" aria-label="Delete {pretty(r.name)}" disabled={!!busy} onclick={() => act(r.name, 'DELETE', path(r.name))}><Icon name="trash" /></button>
          </span>
        {/if}
      {/snippet}
    </DataTable>
  {:else if flows}
    <Empty title="No workflows yet">Ask your AI to set one up, and it will show here.</Empty>
  {/if}
</Sheet>

<style>
  .last { display: inline-flex; align-items: center; gap: var(--space-2); }
  .acts { display: flex; justify-content: flex-end; gap: var(--space-2); }
  .acts button { display: grid; place-items: center; color: var(--fg-muted); transition: color var(--duration-fast); }
  .acts button:hover { color: var(--fg); }
  .acts button:disabled { opacity: var(--opacity-dim); }
</style>
