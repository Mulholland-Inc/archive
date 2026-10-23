import { api, loadMe, spaceLabel } from './api.js';
import { confirmDialog, openDialog } from './app-dialog.js';
import { endpoints, pages } from './endpoints.js';
import { setLocale } from './format.js';
import { registerPasskey } from './sign-in.js';
import { alertBox, button, copy, el, field, listItem, roleLabel, roles, select, textInput, toggle, when } from './ui.js';

const locales = [
  ['', 'Same as this browser'],
  ['en-US', 'English (United States)'],
  ['en-GB', 'English (United Kingdom)'],
  ['nl-NL', 'Nederlands'],
  ['de-DE', 'Deutsch'],
  ['fr-FR', 'Français'],
];

const expiries = [
  ['', 'Never'],
  ['720h', 'In 30 days'],
  ['2160h', 'In 90 days'],
  ['8760h', 'In a year'],
];

const section = (title, actions = []) => {
  const s = document.createElement('section');
  const h = document.createElement('header');
  const div = document.createElement('div');
  div.append(...actions);
  h.append(el('h2', title), div);
  const ul = document.createElement('ul');
  s.append(h, ul);
  return s;
};

const tone = (node, t) => {
  node.dataset.tone = t;
  return node;
};

class SettingsPage extends HTMLElement {
  connectedCallback() {
    this.render();
  }

  async render() {
    const me = await loadMe();
    this.me = me;
    this.alert = alertBox();
    const head = document.createElement('header');
    head.innerHTML = '<div><h1>Settings</h1></div>';
    this.replaceChildren(head, this.alert, this.account(me), await this.spaces(), await this.tokens(), this.preferences());
  }

  fail(err) {
    this.alert.show(err);
    this.alert.scrollIntoView({ block: 'nearest' });
  }

  account(me) {
    const passkey = button('Add a passkey', 'key');
    const status = el('span', '', { role: 'status' });
    passkey.addEventListener('click', async () => {
      try {
        const { passkeys } = await registerPasskey();
        status.textContent = passkeys === 1 ? 'Passkey added' : `Passkey added, you have ${passkeys}`;
        tone(status, 'success');
      } catch (err) {
        if (err.name !== 'NotAllowedError' && err.name !== 'AbortError') this.fail(err);
      }
    });
    const out = button('Sign out');
    out.addEventListener('click', async () => {
      await api(endpoints.signOut, { method: 'POST', redirect: false }).catch(() => {});
      location.assign(pages.signIn);
    });
    const s = section('Account', [out]);
    const who = me.actor.name ? `${me.actor.name}, ${me.actor.email}` : me.actor.email;
    s.querySelector('ul').append(
      listItem([el('p', who), tone(el('p', 'Signed in'), 'muted')]),
      listItem([el('p', 'Passkeys'), tone(el('p', 'Sign in with your fingerprint, face or device instead of an email link.'), 'muted')], [status, passkey]),
    );
    return s;
  }

  async spaces() {
    const spaces = await api(endpoints.spaces);
    const add = button('New space', 'plus');
    add.addEventListener('click', () => this.newSpace());
    const s = section('Spaces', [add]);
    s.querySelector('ul').append(
      ...spaces.map((sp) => {
        const manage = button(['owner', 'admin'].includes(sp.level) ? 'Members' : 'View members', 'space');
        manage.addEventListener('click', () => this.members(sp));
        return listItem([el('p', spaceLabel(sp, this.me.home)), tone(el('p', `You are ${/^[aeiou]/.test(sp.level) ? 'an' : 'a'} ${sp.level}`), 'muted')], [manage]);
      }),
    );
    return s;
  }

  newSpace() {
    const input = textInput('space-name', '', 'household');
    openDialog({
      title: 'New space',
      body: field('Name, in lowercase letters, digits and underscores', input, 'space-name'),
      actions: [
        { label: 'Cancel', value: '' },
        { label: 'Create space', value: 'ok', kind: 'primary' },
      ],
      onAction: () => api(endpoints.spaces, { method: 'POST', body: { name: input.value.trim() } }),
    }).then((v) => v && location.reload());
  }

  async members(sp) {
    const canManage = ['owner', 'admin'].includes(sp.level);
    const options = [...roles.filter((r) => r !== 'owner' || sp.level === 'owner').map((r) => ({ value: r, label: roleLabel(r) })), { value: 'none', label: 'Remove' }];
    const body = document.createElement('div');
    const list = document.createElement('ul');
    list.dataset.list = '';
    const error = alertBox();
    const set = async (email, role) => {
      error.show('');
      try {
        draw(await api(endpoints.members(sp.name), { method: 'POST', body: { email, role } }));
        return true;
      } catch (err) {
        error.show(err);
        return false;
      }
    };
    const draw = (members) =>
      list.replaceChildren(
        ...members.map((m) => {
          const who = [el('p', m.name || m.email)];
          if (m.name) who.push(tone(el('p', m.email), 'muted'));
          if (!canManage || (m.level === 'owner' && sp.level !== 'owner')) return listItem(who, [el('span', roleLabel(m.level))]);
          const role = select(`Role of ${m.email}`, options, m.level);
          role.addEventListener('change', () => set(m.email, role.value));
          return listItem(who, [role]);
        }),
      );
    draw(await api(endpoints.members(sp.name)));
    body.append(list, error);
    if (canManage) {
      const email = textInput('member-email', '', 'name@example.com');
      email.type = 'email';
      const role = select('Role', options.slice(0, -1), 'editor');
      const add = button('Add', 'plus');
      const row = document.createElement('div');
      row.dataset.inline = '';
      row.append(email, role, add);
      add.addEventListener('click', async () => {
        if (await set(email.value.trim(), role.value)) email.value = '';
      });
      email.addEventListener('keydown', (e) => {
        if (e.key !== 'Enter') return;
        e.preventDefault();
        add.click();
      });
      body.append(field('Add someone who already has an Archive account', row, 'member-email'));
    }
    await openDialog({ title: `Members of ${spaceLabel(sp, this.me.home)}`, body, actions: [{ label: 'Done', value: 'done', kind: 'primary' }] });
    this.render();
  }

  async tokens() {
    const tokens = await api(endpoints.tokens);
    const add = button('New token', 'plus');
    add.addEventListener('click', () => this.newToken());
    const s = section('API tokens', [add]);
    const ul = s.querySelector('ul');
    if (!tokens.length) ul.append(listItem([tone(el('p', 'No tokens yet. Make one for an agent, a script or Terraform.'), 'muted')]));
    ul.append(
      ...tokens.map((t) => {
        const access = t.full ? 'All your rights' : Object.entries(t.spaces).map(([n, r]) => `${roleLabel(r)} in ${spaceLabel(this.me.spaces.find((x) => x.name === n) ?? { name: n }, this.me.home)}`).join(', ');
        const dates = [`Made ${when(t.created_at, { dateStyle: 'medium' })}`, t.expires_at && `expires ${when(t.expires_at, { dateStyle: 'medium' })}`].filter(Boolean).join(', ');
        const revoke = button('Revoke', 'trash');
        revoke.addEventListener('click', async () => {
          if (!(await confirmDialog({ title: `Revoke ${t.name || 'this token'}?`, text: 'Anything using it stops working at once.', action: 'Revoke' }))) return;
          try {
            await api(endpoints.token(t.id), { method: 'DELETE' });
            this.render();
          } catch (err) {
            this.fail(err);
          }
        });
        return listItem([el('p', t.name || 'Unnamed token'), tone(el('p', access), 'muted'), tone(el('p', dates), 'muted')], [revoke]);
      }),
    );
    return s;
  }

  async newToken() {
    const spaces = this.me.spaces;
    const name = textInput('token-name', '', 'Claude');
    const full = toggle('All my rights, including managing spaces, members and tokens', false);
    const grants = document.createElement('div');
    grants.dataset.stack = '';
    const pickers = spaces.map((sp) => {
      const options = [{ value: '', label: 'No access' }, ...roles.slice(roles.indexOf(sp.level)).map((r) => ({ value: r, label: roleLabel(r) }))];
      const picker = select(`Access to ${spaceLabel(sp, this.me.home)}`, options, sp.level === 'viewer' ? 'viewer' : 'editor');
      grants.append(field(spaceLabel(sp, this.me.home), picker));
      return [sp.name, picker];
    });
    full.addEventListener('change', () => (grants.hidden = full.checked));
    const expires = select('Expires', expiries.map(([value, text]) => ({ value, label: text })), '');
    const body = document.createElement('div');
    body.append(field('Name', name, 'token-name'), full, grants, field('Expires', expires));
    let made;
    const ok = await openDialog({
      title: 'New API token',
      body,
      actions: [
        { label: 'Cancel', value: '' },
        { label: 'Make token', value: 'ok', kind: 'primary' },
      ],
      onAction: async () => {
        const chosen = Object.fromEntries(pickers.map(([n, p]) => [n, p.value]).filter(([, v]) => v));
        if (!full.checked && !Object.keys(chosen).length) throw new Error('Give the token access to at least one space.');
        made = await api(endpoints.tokens, {
          method: 'POST',
          body: { name: name.value.trim(), full: full.checked, spaces: full.checked ? undefined : chosen, expires: expires.value || undefined },
        });
      },
    });
    if (!ok) return;
    const secret = el('p', made.token);
    secret.dataset.secret = '';
    const copyButton = button('Copy token', 'link');
    copyButton.addEventListener('click', () => copy(copyButton, made.token));
    const shown = document.createElement('div');
    shown.append(el('p', 'Copy it now. Archive keeps only a fingerprint, so you will not see it again.'), secret, copyButton);
    await openDialog({ title: 'Your new token', body: shown, actions: [{ label: 'Done', value: 'done', kind: 'primary' }] });
    this.render();
  }

  preferences() {
    const s = section('Preferences');
    const picker = select('Dates and amounts', locales.map(([value, text]) => ({ value, label: text })), localStorage.getItem('archive.locale') ?? '');
    picker.addEventListener('change', () => setLocale(picker.value));
    s.querySelector('ul').append(listItem([el('p', 'Dates and amounts')], [picker]));
    return s;
  }
}

customElements.define('settings-page', SettingsPage);
