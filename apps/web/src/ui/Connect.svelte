<script module>
  // Front end only: nothing here connects to anything yet. Connected apps are remembered in this browser.
  // The list is the one on archive.dev: the AI apps on /agents and the connectors on /connectors.
  const APPS = [
    ['Your AI', 'Your AI', ['ChatGPT', 'Claude', 'Gemini', 'Grok', 'Claude Code', 'Cursor']],
    ['Mail and calendar', 'Mail', ['Gmail', 'Google Calendar', 'Outlook mail', 'Outlook calendar']],
    ['Files', 'Files', ['Google Drive', 'SharePoint and OneDrive', 'Dropbox', 'Box']],
    ['Chat', 'Chat', ['Slack', 'Microsoft Teams', 'Google Chat']],
    ['Docs and knowledge', 'Docs', ['Notion', 'Confluence']],
    ['Work tracking and code', 'Work', ['Jira', 'Linear', 'Asana', 'GitHub', 'GitLab']],
    ['CRM and support', 'CRM', ['HubSpot', 'Salesforce', 'Pipedrive', 'Zendesk', 'Intercom']],
    ['Finance and meetings', 'Finance', ['Stripe', 'Xero', 'QuickBooks Online', 'Exact Online', 'Zoom']],
  ];
  const RECOMMENDED = ['ChatGPT', 'Claude', 'Google Drive', 'SharePoint and OneDrive', 'Gmail', 'Slack'];
  const LOGO = { 'SharePoint and OneDrive': 'OneDrive', 'Outlook mail': 'Microsoft', 'Outlook calendar': 'Microsoft', 'QuickBooks Online': 'QuickBooks', 'Google Chat': 'Google Workspace' };
</script>

<script>
  import { BRANDS } from '#kit/brands.js';
  import Icon from '#kit/Icon.svelte';

  let { workspace = '', onclose } = $props();
  let query = $state('');
  let group = $state('All');
  let busy = $state('');
  let on = $state(read());

  function key() {
    return `connected:${workspace}`;
  }

  function read() {
    try {
      return JSON.parse(localStorage[`connected:${workspace}`] ?? '[]');
    } catch {
      return [];
    }
  }

  function save() {
    try {
      localStorage[key()] = JSON.stringify(on);
    } catch {}
    dispatchEvent(new CustomEvent('connected'));
  }

  function toggle(app) {
    if (on.includes(app)) {
      on = on.filter((a) => a !== app);
      return save();
    }
    busy = app;
    setTimeout(() => {
      on = [...on, app];
      busy = '';
      save();
    }, 900);
  }

  const logo = (app) => BRANDS[LOGO[app] ?? app];
  const words = $derived(query.trim().toLowerCase());
  const shown = $derived([...(group === 'All' && !words ? [['Recommended for you', '', RECOMMENDED]] : []), ...APPS]
    .filter(([, short]) => group === 'All' || short === group)
    .map(([title, , apps]) => [title, apps.filter((a) => a.toLowerCase().includes(words))])
    .filter(([, apps]) => apps.length));

  const esc = (e) => e.key === 'Escape' && onclose();
</script>

<svelte:window onkeydown={esc} />

<button class="scrim" type="button" aria-label="Close" onclick={onclose}></button>
<div class="sheet" role="dialog" aria-modal="true" aria-label="Connect apps">
  <button class="close" type="button" aria-label="Close" onclick={onclose}><Icon name="x" /></button>
  <header>
    <h2>Connect apps</h2>
    <span class="count">
      <span class="stack">{#each on.slice(0, 4) as app (app)}<span class="dot">{#if logo(app)}<svg viewBox={logo(app)[0]} fill="currentColor" aria-hidden="true">{@html logo(app)[1]}</svg>{:else}{app[0]}{/if}</span>{/each}</span>
      {on.length} connected
    </span>
    <span class="demo">Demo</span>
  </header>
  <label class="search"><Icon name="search" /><input bind:value={query} placeholder="Search across apps" aria-label="Search apps" /></label>
  <div class="chips">
    {#each ['All', ...APPS.map(([, short]) => short)] as g (g)}
      <button type="button" class:on={group === g} onclick={() => (group = g)}>{g}</button>
    {/each}
  </div>
  <div class="scroll">
    {#each shown as [title, apps] (title)}
      <section>
        <h3>{title}</h3>
        <div class="grid">
          {#each apps as app (app)}
            <div class="app">
              <span class="logo">
                {#if logo(app)}<svg viewBox={logo(app)[0]} fill="currentColor" aria-hidden="true">{@html logo(app)[1]}</svg>{:else}{app[0]}{/if}
              </span>
              <span class="name">{app}</span>
              <button class="connect" class:done={on.includes(app)} type="button" disabled={busy === app} aria-label="{on.includes(app) ? 'Disconnect' : 'Connect'} {app}" onclick={() => toggle(app)}>
                {busy === app ? 'Connecting' : on.includes(app) ? 'Connected' : 'Connect'}
              </button>
            </div>
          {/each}
        </div>
      </section>
    {:else}
      <p>Nothing matches.</p>
    {/each}
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: calc(var(--z-nav) + 1); width: 100%; background: rgb(0 0 0 / 35%); cursor: default; }
  .sheet { position: fixed; top: 50%; left: 50%; z-index: calc(var(--z-nav) + 2); display: flex; flex-direction: column; gap: var(--space-3); width: min(760px, calc(100vw - 2 * var(--space-3))); height: min(640px, calc(100svh - 2 * var(--space-3))); padding: var(--space-5); transform: translate(-50%, -50%); border-radius: var(--radius-panel); background: var(--surface); box-shadow: 0 12px 40px rgb(0 0 0 / 25%); }
  .close { position: absolute; top: var(--space-3); right: var(--space-3); display: grid; place-items: center; width: 32px; height: 32px; color: var(--fg-muted); }
  .close:hover { color: var(--fg); }
  header { display: flex; align-items: center; gap: var(--space-2); padding-right: var(--space-6); }
  h2 { font: var(--weight-medium) var(--text-lg) / var(--leading-tight) var(--font-serif); }
  .count { display: flex; align-items: center; gap: var(--space-1-5); margin-left: auto; color: var(--fg-muted); font-size: var(--text-meta); }
  .stack { display: flex; }
  .dot { display: grid; place-items: center; width: 22px; height: 22px; margin-left: -6px; border-radius: 30%; background: var(--fill-strong); color: var(--fg); font-size: 11px; }
  .dot svg { width: 13px; height: 13px; }
  .demo { padding: 2px var(--space-1-5); border-radius: var(--radius-full); background: var(--fill); color: var(--fg-muted); font-size: var(--text-meta); }
  .search { display: flex; flex: none; align-items: center; gap: var(--space-2); padding: var(--space-2) var(--space-3); border-radius: var(--radius-full); background: var(--fill-faint); color: var(--fg-muted); }
  .search input { flex: 1; min-width: 0; padding: 0; color: var(--fg); outline: 0; }
  .chips { display: flex; flex: none; flex-wrap: wrap; gap: var(--space-1); }
  .chips button { padding: var(--space-1) var(--space-3); border-radius: var(--radius-full); background: var(--fill-faint); color: var(--fg-soft); font-size: var(--text-table); }
  .chips .on { background: var(--fg); color: var(--bg); }
  .scroll { display: flex; flex: 1; flex-direction: column; gap: var(--space-5); min-height: 0; overflow-y: auto; }
  h3 { margin-bottom: var(--space-2); font-size: var(--text-table); font-weight: var(--weight-medium); }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: var(--space-2) var(--space-4); }
  .app { display: flex; align-items: center; gap: var(--space-3); min-width: 0; }
  .logo { display: grid; place-items: center; flex: none; width: 40px; height: 40px; border-radius: 25%; background: var(--fill-faint); color: var(--fg); font-weight: var(--weight-medium); }
  .logo svg { width: 22px; height: 22px; }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .connect { flex: none; padding: var(--space-1) var(--space-3); border-radius: var(--radius-full); background: var(--fill); font-size: var(--text-table); transition: background var(--duration-fast); }
  .connect:hover { background: var(--fill-strong); }
  .connect.done { background: none; color: var(--fg-muted); }
  .connect:disabled { color: var(--fg-muted); }
  p { color: var(--fg-muted); text-align: center; }
</style>
