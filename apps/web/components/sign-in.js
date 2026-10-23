import { api, tidy } from './api.js';
import { endpoints, pages } from './endpoints.js';
import { label } from './ontology.js';

const next = () => {
  const n = new URLSearchParams(location.search).get('next') ?? '';
  return n.startsWith('/') && !n.startsWith('//') ? n : pages.home;
};

const providerNames = { google: 'Google', microsoft: 'Microsoft' };

export async function registerPasskey() {
  if (!window.PublicKeyCredential?.parseCreationOptionsFromJSON) throw new Error('This browser cannot make passkeys.');
  const options = await api(endpoints.passkeyRegister, { method: 'POST' });
  const credential = await navigator.credentials.create({ publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(options.publicKey) });
  return api(endpoints.passkeyRegisterFinish, { method: 'POST', body: credential.toJSON() });
}

class SignIn extends HTMLElement {
  connectedCallback() {
    const params = new URLSearchParams(location.search);
    if (params.has('setup')) return this.setup(params.get('setup'));
    this.innerHTML = `
      <a href="${pages.home}" aria-label="Archive home"><archive-wordmark></archive-wordmark></a>
      <h1>Sign in to Archive</h1>
      <form novalidate data-email>
        <label for="sign-in-email">Email</label>
        <input id="sign-in-email" name="email" type="email" autocomplete="email webauthn" inputmode="email" placeholder="you@example.com">
        <button class="button button-primary button-sm"><app-icon name="mail"></app-icon><span>Email me a sign-in link</span></button>
      </form>
      <p data-sent role="status" hidden></p>
      <div data-others>
        <button type="button" class="button button-secondary button-sm" data-passkey><app-icon name="key"></app-icon><span>Sign in with a passkey</span></button>
      </div>
      <p role="alert" hidden></p>`;
    const alert = this.querySelector('[role=alert]');
    this.fail = (message) => {
      alert.textContent = message;
      alert.hidden = !message;
    };
    if (params.get('error')) this.fail(tidy(params.get('error')));
    const methods = api(endpoints.methods, { redirect: false }).catch(() => ({ providers: [], mail: true }));
    methods.then(({ providers }) => {
      this.querySelector('[data-others]').append(
        ...providers.map((p) => {
          const a = document.createElement('a');
          a.className = 'button button-secondary button-sm';
          a.href = endpoints.oidc(p);
          a.innerHTML = `<app-icon name="${providerNames[p] ? p : 'key'}"></app-icon><span></span>`;
          a.querySelector('span').textContent = `Continue with ${providerNames[p] ?? label(p)}`;
          return a;
        }),
      );
    });
    const form = this.querySelector('[data-email]');
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      this.fail('');
      const email = form.email.value.trim();
      if (!/^[^\s@]+@[^\s@]+$/.test(email)) return this.fail('Enter your email address.');
      const button = form.querySelector('button');
      button.disabled = true;
      try {
        await api(endpoints.magic, { method: 'POST', body: { email }, redirect: false });
        const { mail } = await methods;
        const sent = this.querySelector('[data-sent]');
        sent.textContent = mail
          ? `If ${email} has an account, a sign-in link is on its way. Open it within 15 minutes.`
          : `If ${email} has an account, its sign-in link is in the Archive server log, because this Archive has no mail server set up.`;
        sent.hidden = false;
        form.hidden = true;
      } catch (err) {
        this.fail(err.message);
      }
      button.disabled = false;
    });
    this.querySelector('[data-passkey]').addEventListener('click', () => this.passkey());
  }

  async passkey() {
    this.fail('');
    if (!window.PublicKeyCredential?.parseRequestOptionsFromJSON) return this.fail('This browser cannot use passkeys.');
    try {
      const options = await api(endpoints.passkeyLogin, { method: 'POST', redirect: false });
      const credential = await navigator.credentials.get({ publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(options.publicKey) });
      await api(endpoints.passkeyLoginFinish, { method: 'POST', body: credential.toJSON(), redirect: false });
      location.assign(next());
    } catch (err) {
      if (err.name !== 'NotAllowedError' && err.name !== 'AbortError') this.fail(err.message);
    }
  }

  setup(token) {
    this.innerHTML = `
      <a href="${pages.home}" aria-label="Archive home"><archive-wordmark></archive-wordmark></a>
      <h1>Set up Archive</h1>
      <p>Create the first owner. You can add more people later.</p>
      <form novalidate data-setup>
        <div><label for="setup-email">Email</label><input id="setup-email" name="email" type="email" autocomplete="email" inputmode="email" placeholder="you@example.com"></div>
        <div><label for="setup-name">Name, optional</label><input id="setup-name" name="name" type="text" autocomplete="name"></div>
        <button class="button button-primary button-sm"><span>Create the first owner</span></button>
      </form>
      <p role="alert" hidden></p>`;
    const alert = this.querySelector('[role=alert]');
    const form = this.querySelector('form');
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      alert.hidden = true;
      const email = form.email.value.trim();
      if (!/^[^\s@]+@[^\s@]+$/.test(email)) {
        alert.textContent = 'Enter your email address.';
        alert.hidden = false;
        return;
      }
      try {
        await api(endpoints.setup, { method: 'POST', body: { token, email, name: form.name.value.trim() }, redirect: false });
        location.assign(pages.home);
      } catch (err) {
        alert.textContent = err.status === 401 ? 'This setup link is invalid or was used already. Find the current one in the Archive server log.' : err.message;
        alert.hidden = false;
      }
    });
  }
}

customElements.define('sign-in', SignIn);
