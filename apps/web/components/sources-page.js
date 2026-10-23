import { api, currentSpace, tidy } from './api.js';
import { confirmDialog, openDialog } from './app-dialog.js';
import { endpoints, pages } from './endpoints.js';
import { label, loadSpec, userTypes } from './ontology.js';
import { alertBox, button, el, field, header, listItem, select, textInput, toggle, when } from './ui.js';

const kinds = [
  ['gmail', 'Gmail'],
  ['drive', 'Google Drive'],
  ['imap', 'Mail over IMAP'],
  ['folder', 'Folder on the server'],
  ['external', 'External connector'],
];

const schedules = [
  ['', 'Only when I run it'],
  ['15m', 'Every 15 minutes'],
  ['1h', 'Every hour'],
  ['6h', 'Every 6 hours'],
  ['24h', 'Every day'],
];

const statuses = {
  new: ['Not run yet', 'muted'],
  needs_auth: ['Waiting for Google sign-in', 'warning'],
  running: ['Syncing', 'warning'],
  ok: ['Up to date', 'success'],
  error: ['Failed', 'destructive'],
};

const kindLabel = (k) => kinds.find(([v]) => v === k)?.[1] ?? k;

const scheduleLabel = (s) => schedules.find(([v]) => v === s)?.[1] ?? (s ? `Every ${s}` : schedules[0][1]);

const tone = (node, t) => {
  node.dataset.tone = t;
  return node;
};

const words = (text) =>
  text
    .split(/[\s,]+/)
    .map((w) => w.trim())
    .filter(Boolean);

const json = (input, what) => {
  const text = input.value.trim();
  if (!text) return {};
  try {
    const v = JSON.parse(text);
    if (v && typeof v === 'object' && !Array.isArray(v)) return v;
  } catch {}
  throw new Error(`${what} must be a JSON object, like {"key": "value"}.`);
};

const paused = (s) => localStorage.getItem(`archive.paused.${s.id}`);

class SourcesPage extends HTMLElement {
  async connectedCallback() {
    [this.spec, this.space] = await Promise.all([loadSpec(), currentSpace()]);
    this.alert = alertBox();
    const error = new URLSearchParams(location.search).get('error');
    if (error) {
      this.alert.show(tidy(error));
      history.replaceState(null, '', pages.sources);
    }
    this.list = document.createElement('ul');
    this.list.dataset.list = '';
    const add = button('Add source', 'plus', 'primary');
    add.addEventListener('click', () => this.add());
    this.replaceChildren(header('Sources', [add]), this.alert, this.list);
    this.load();
  }

  typeOptions() {
    return [{ value: '', label: 'Keep raw, map nothing' }, ...userTypes(this.spec).map((t) => ({ value: t, label: `Map into ${label(t).toLowerCase()}` }))];
  }

  async load() {
    const all = await api(endpoints.sources);
    const sources = all.filter((s) => s.space === this.space?.id);
    if (!sources.length) {
      this.list.replaceChildren(listItem([tone(el('p', 'No sources in this space yet. Connect Gmail, Drive, a mailbox, a folder or any Airbyte or Singer connector, and Archive keeps a copy of everything it finds.'), 'muted')]));
      return;
    }
    this.list.replaceChildren(...sources.map((s) => this.item(s)));
    clearTimeout(this.timer);
    if (sources.some((s) => s.status === 'running')) this.timer = setTimeout(() => this.load(), 2000);
  }

  item(s) {
    const [word, t] = statuses[s.status] ?? [s.status, 'muted'];
    const about = [kindLabel(s.kind), s.type ? `mapped into ${label(s.type).toLowerCase()}` : 'kept raw', s.schedule ? scheduleLabel(s.schedule).toLowerCase() : 'run by hand'].join(', ');
    const runs = [s.last_run_at && `last run ${when(s.last_run_at)}`, s.next_run_at && `next run ${when(s.next_run_at)}`].filter(Boolean).join(', ').replace(/^./, (c) => c.toUpperCase());
    const main = [el('p', s.name), tone(el('p', about), 'muted')];
    if (runs) main.push(tone(el('p', runs), 'muted'));
    if (s.error) main.push(tone(el('p', s.error), 'destructive'));
    const run = button('Run now', 'play');
    run.disabled = s.status === 'running' || s.status === 'needs_auth';
    run.addEventListener('click', () => this.run(s, run));
    const pause = s.schedule ? button('Pause', 'pause') : paused(s) ? button('Resume', 'play') : null;
    pause?.addEventListener('click', () => this.pause(s));
    const edit = button('Edit', 'edit');
    edit.addEventListener('click', () => this.edit(s));
    const remove = button('Remove', 'trash');
    remove.addEventListener('click', () => this.remove(s));
    return listItem(main, [tone(el('span', word), t), run, pause, edit, remove]);
  }

  async run(s, b) {
    this.alert.show('');
    b.disabled = true;
    b.querySelector('span').textContent = 'Syncing';
    try {
      const res = await api(endpoints.runSource(s.id), { method: 'POST' });
      if (res.error) this.alert.show(tidy(res.error));
    } catch (err) {
      this.alert.show(err);
    }
    this.load();
  }

  async patch(s, body) {
    try {
      await api(endpoints.source(s.id), { method: 'PATCH', body });
      this.load();
    } catch (err) {
      this.alert.show(err);
    }
  }

  pause(s) {
    if (s.schedule) {
      localStorage.setItem(`archive.paused.${s.id}`, s.schedule);
      return this.patch(s, { schedule: '' });
    }
    const schedule = paused(s);
    localStorage.removeItem(`archive.paused.${s.id}`);
    return this.patch(s, { schedule });
  }

  edit(s) {
    const type = select('Map into', this.typeOptions(), s.type ?? '');
    const schedule = select('Schedule', schedules.map(([value, text]) => ({ value, label: text })), schedules.some(([v]) => v === s.schedule) ? s.schedule : '');
    const rate = textInput('source-rate', String(s.rate));
    rate.inputMode = 'decimal';
    const body = document.createElement('div');
    body.append(field('Map into', type), field('Schedule', schedule), field('Requests per second', rate, 'source-rate'));
    openDialog({
      title: `Edit ${s.name}`,
      body,
      actions: [
        { label: 'Cancel', value: '' },
        { label: 'Save', value: 'ok', kind: 'primary' },
      ],
      onAction: async () => {
        localStorage.removeItem(`archive.paused.${s.id}`);
        await api(endpoints.source(s.id), { method: 'PATCH', body: { type: type.value, schedule: schedule.value, rate: Number(rate.value) || 0 } });
      },
    }).then((v) => v && this.load());
  }

  async remove(s) {
    const ok = await confirmDialog({ title: `Remove ${s.name}?`, text: 'Archive stops syncing it. What it already fetched, and the records made from it, stay.', action: 'Remove' });
    if (!ok) return;
    try {
      await api(endpoints.source(s.id), { method: 'DELETE' });
      localStorage.removeItem(`archive.paused.${s.id}`);
      this.load();
    } catch (err) {
      this.alert.show(err);
    }
  }

  add() {
    const kind = select('Kind', kinds.map(([value, text]) => ({ value, label: text })), 'folder');
    const name = textInput('source-name', '', 'inbox');
    const type = select('Map into', this.typeOptions(), '');
    const schedule = select('Schedule', schedules.map(([value, text]) => ({ value, label: text })), '1h');
    const rate = textInput('source-rate', '5');
    rate.inputMode = 'decimal';
    const inputs = {
      query: textInput('source-query', '', 'newer_than:30d'),
      host: textInput('source-host', '', 'imap.fastmail.com:993'),
      username: textInput('source-username', '', 'you@example.com'),
      password: Object.assign(textInput('source-password'), { type: 'password', autocomplete: 'new-password' }),
      mailboxes: textInput('source-mailboxes', '', 'INBOX, Archive'),
      starttls: toggle('Use STARTTLS instead of TLS', false),
      path: textInput('source-path', '', '/data/inbox'),
      command: textInput('source-command', '', 'docker run -i airbyte/source-faker'),
      protocol: select('Protocol', [{ value: 'airbyte', label: 'Airbyte' }, { value: 'singer', label: 'Singer' }], 'airbyte'),
      streams: textInput('source-streams', '', 'users, purchases'),
      config: textInput('source-config', '', '{"count": 100}'),
      secrets: Object.assign(textInput('source-secrets', '', '{"api_key": "..."}'), { type: 'password' }),
    };
    const groups = {
      gmail: [field('Which mail, as a Gmail search, optional', inputs.query, 'source-query'), el('p', 'After you add it, Google asks you to allow Archive to read your mail.')],
      drive: [el('p', 'After you add it, Google asks you to allow Archive to read your files.')],
      imap: [
        field('Server', inputs.host, 'source-host'),
        field('Username', inputs.username, 'source-username'),
        field('Password, stored encrypted', inputs.password, 'source-password'),
        field('Mailboxes, optional', inputs.mailboxes, 'source-mailboxes'),
        inputs.starttls,
      ],
      folder: [field('Folder path on the Archive server', inputs.path, 'source-path')],
      external: [
        field('Command', inputs.command, 'source-command'),
        field('Protocol', inputs.protocol),
        field('Streams, optional', inputs.streams, 'source-streams'),
        field('Settings as JSON, optional', inputs.config, 'source-config'),
        field('Secrets as JSON, stored encrypted, optional', inputs.secrets, 'source-secrets'),
      ],
    };
    const specific = document.createElement('div');
    specific.dataset.stack = '';
    const sync = () => specific.replaceChildren(...groups[kind.value]);
    kind.addEventListener('change', sync);
    sync();
    const body = document.createElement('div');
    body.append(field('Kind', kind), field('Name, in lowercase letters, digits, dots and dashes', name, 'source-name'), specific, field('Map into', type), field('Schedule', schedule), field('Requests per second', rate, 'source-rate'));
    let made;
    openDialog({
      title: 'Add a source',
      body,
      actions: [
        { label: 'Cancel', value: '' },
        { label: 'Add source', value: 'ok', kind: 'primary' },
      ],
      onAction: async () => {
        const k = kind.value;
        const config = {};
        const credentials = {};
        if (k === 'gmail' && inputs.query.value.trim()) config.query = inputs.query.value.trim();
        if (k === 'imap') {
          Object.assign(config, { host: inputs.host.value.trim(), username: inputs.username.value.trim(), starttls: inputs.starttls.checked });
          if (inputs.mailboxes.value.trim()) config.mailboxes = words(inputs.mailboxes.value);
          if (inputs.password.value) credentials.password = inputs.password.value;
        }
        if (k === 'folder') config.path = inputs.path.value.trim();
        if (k === 'external') {
          Object.assign(config, { command: inputs.command.value.trim().split(/\s+/).filter(Boolean), protocol: inputs.protocol.value, config: json(inputs.config, 'Settings') });
          if (inputs.streams.value.trim()) config.streams = words(inputs.streams.value);
          Object.assign(credentials, json(inputs.secrets, 'Secrets'));
        }
        made = await api(endpoints.sources, {
          method: 'POST',
          body: { kind: k, name: name.value.trim(), space: this.space?.name, type: type.value, schedule: schedule.value, rate: Number(rate.value) || 0, config, credentials },
        });
      },
    }).then((v) => {
      if (!v) return;
      if (made.auth_url) location.assign(made.auth_url);
      else this.load();
    });
  }
}

customElements.define('sources-page', SourcesPage);
