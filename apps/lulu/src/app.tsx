import { StrictMode, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';
import { boot, clearTurns, create, get, list, listen, methods, moved, verifyCode, patch, patchWorkspace, remove, setWorkspace, signOut, turn, turns, upload, useRead, type Row } from './api';
import { useContext } from 'react';
import { Button, Chip, Combo, Confirm, Dialog, Field, FilePicker, FloatingProvider, Icon, IconButton, List, Loading, Note, Page, PageState, Problem, Section, ShellCtx, Table, Tabs, Tag, Toasts, Toolbar, toast, type IconName } from './ui';
import { NavCtx, isOpen, needsYou, show, useNav } from './model';
import { BuildPage, Builds, NewBuild, bookRead } from './builds';
import { ProductPage, Products, Templates, VendorPage, Vendors } from './catalog';
import { TemplatePage } from './template';
import { VendorPortal } from './portal';

const look = localStorage.getItem('look');
if (look === 'light' || look === 'dark') document.documentElement.dataset.theme = look;
else document.documentElement.dataset.theme = look === 'system' ? '' : 'light';

const NAV: { path: string; label: string; icon: IconName }[] = [
  { path: '/builds', label: 'Builds', icon: 'box' }, { path: '/products', label: 'Products', icon: 'list' }, { path: '/templates', label: 'Templates', icon: 'layout' }, { path: '/vendors', label: 'Vendors', icon: 'globe' }
];

function SignIn() {
  const [ways, setWays] = useState<{ id: string; name?: string }[] | null>(null);
  const [asking, setAsking] = useState(false);
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  useEffect(() => { methods().then(m => { setWays(m.providers); setAsking(m.code); }, e => setError((e as Error).message)); }, []);
  const api = import.meta.env.VITE_API ?? `https://api.${location.hostname.split('.').slice(1).join('.')}`;
  const verify = () => verifyCode(code).then(() => location.reload(), e => setError((e as Error).message));
  return (
    <main className="gate">
      <section className="card gate__card">
        <h1 className="gate__title">Lulu &amp; Georgia</h1>
        {error && <Problem>{error}</Problem>}
        {asking && <Field label="Code from your app" value={code} autoFocus onChange={setCode} onEnter={verify} />}
        {asking && <Button variant="primary" disabled={!code.trim()} onClick={verify}>Continue</Button>}
        {!asking && ways?.map(p => <a key={p.id} className="btn" href={`${api}/auth/oidc/${p.id}?app=${encodeURIComponent(location.origin)}&next=${encodeURIComponent(location.pathname)}`}>{p.name ?? p.id}</a>)}
        {!asking && ways && !ways.length && <Note block>No way to sign in is set up here.</Note>}
      </section>
    </main>
  );
}

function usePath() {
  const [path, setPath] = useState(location.pathname);
  useEffect(() => { const on = () => setPath(location.pathname); addEventListener('popstate', on); return () => removeEventListener('popstate', on); }, []);
  const go = useCallback((to: string) => { history.pushState({ depth: (history.state?.depth ?? 0) + 1 }, '', to); setPath(to.split('?')[0]); }, []);
  const swap = useCallback((to: string) => { history.replaceState(history.state, '', to); setPath(to.split('?')[0]); }, []);
  return { path, go, swap };
}

function Shell({ who, name }: { who: Row; name: string }) {
  const { path, go, swap } = usePath();
  const [rail, setRail] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const [theme, setTheme] = useState(() => localStorage.getItem('look') ?? 'light');
  const pick = (t: string) => { localStorage.setItem('look', t); document.documentElement.dataset.theme = t === 'system' ? '' : t; setTheme(t); };
  const dark = theme === 'dark' || (theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
  useEffect(() => { listen(); }, []);
  useEffect(() => { document.title = name; setNavOpen(false); }, [path, name]);
  const book = useRead('build_book', bookRead, ['build', 'build_row', 'build_image', 'build_activity', 'vendor', 'template']);
  const waiting = (book.data ?? []).filter(r => isOpen(r) && needsYou(r));
  const [a, b, c] = path.split('/').filter(Boolean).map(decodeURIComponent);
  const me = String(who.me.actor?.email ?? '');
  const page: ReactNode =
    !a ? <Agent who={who} agents={who.agents ?? []} waiting={waiting} waitingError={book.error} />
      : a === 'builds' && b === 'new' ? <NewBuild key={c ?? ''} vendor={c} />
        : a === 'builds' && b ? <BuildPage key={b} id={b} />
          : a === 'builds' ? <Builds me={me} />
            : a === 'products' && b ? <ProductPage key={b} id={b} />
              : a === 'products' ? <Products />
                : a === 'templates' && b ? <TemplatePage key={b} id={b} />
                  : a === 'templates' ? <Templates />
                    : a === 'vendors' && b ? <VendorPage key={b} id={b} />
                      : a === 'vendors' ? <Vendors />
                        : a === 'team' ? <Team myRole={String(who.me.role)} />
                          : a === 'settings' ? <Settings who={who} name={name} theme={theme} pick={pick} />
                            : <PageState title="Not found" error="There is no such page here." />;
  const item = (p: string, label: string, icon: IconName, count?: number) => (
    <button key={p} type="button" className={`nav-item ${(p === '/' ? !a : path.startsWith(p)) ? 'nav-item--on' : ''}`} title={label} onClick={() => go(p)}><Icon name={icon} /><span className="nav-item__label">{label}</span>{!!count && <span className="nav-item__count">{count}</span>}</button>
  );
  const nav = useMemo(() => ({ go, swap, brand: name }), [go, swap, name]);
  const phone = () => matchMedia('(max-width: 900px)').matches;
  const lead = <IconButton icon="sidebar" label={rail || phone() ? 'Open sidebar' : 'Close sidebar'} onClick={() => (phone() ? setNavOpen(true) : setRail(r => !r))} />;
  return (
    <NavCtx.Provider value={nav}>
      <div className={`app ${navOpen ? 'app--nav' : ''}`}>
        <aside className={`sidebar ${rail && !navOpen ? 'sidebar--rail' : ''}`}>
          <div className="sidebar__ws" title={name}><Icon name="building" /><span className="nav-item__label">{name}</span></div>
          <nav className="sidebar__group">{item('/', 'Agent', 'chat', waiting.length)}</nav>
          <div className="sidebar__scroll">
            <nav className="sidebar__group"><div className="sidebar__label">Ecommerce</div>{NAV.map(n => item(n.path, n.label, n.icon))}</nav>
            <nav className="sidebar__group"><div className="sidebar__label">Platform</div>{item('/team', 'Team', 'badge')}</nav>
          </div>
          <nav className="sidebar__group">{item('/settings', 'Settings', 'settings')}</nav>
          <div className="sidebar__me">
            <Icon name="user" /><span className="nav-item__label">{String(who.me.actor?.name || me)}</span>
            <IconButton icon={dark ? 'sun' : 'moon'} label={dark ? 'Light' : 'Dark'} onClick={() => pick(dark ? 'light' : 'dark')} />
            <IconButton icon="signOut" label="Sign out" onClick={() => void signOut()} />
          </div>
        </aside>
        <div className="scrim" onClick={() => setNavOpen(false)} />
        <main className="pane" key={path}>
          <ShellCtx.Provider value={{ lead, back: (history.state?.depth ?? 0) > 0 ? () => history.back() : undefined }}>{page}</ShellCtx.Provider>
        </main>
      </div>
      <Toasts />
    </NavCtx.Provider>
  );
}

const inline = (text: string): ReactNode[] => text.split(/(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`|\[[^\]]+\]\([^)\s]+\))/).map((p, i) => {
  if (/^\*\*.+\*\*$/.test(p)) return <strong key={i}>{p.slice(2, -2)}</strong>;
  if (/^\*.+\*$/.test(p)) return <em key={i}>{p.slice(1, -1)}</em>;
  if (/^`.+`$/.test(p)) return <code key={i}>{p.slice(1, -1)}</code>;
  const link = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(p);
  return link ? <a key={i} href={link[2]} target="_blank" rel="noreferrer">{link[1]}</a> : p;
});
function Markdown({ text }: { text: string }) {
  const out: ReactNode[] = [];
  const lines = text.split('\n');
  const cells = (l: string) => l.replace(/^\s*\||\|\s*$/g, '').split('|').map(c => c.trim());
  for (let i = 0; i < lines.length;) {
    const l = lines[i];
    if (!l.trim()) { i++; continue; }
    if (/^\s*\|.*\|\s*$/.test(l) && /^\s*\|?[\s:|-]+\|?\s*$/.test(lines[i + 1] ?? '')) {
      const head = cells(l);
      const rows: string[][] = [];
      for (i += 2; /^\s*\|.*\|\s*$/.test(lines[i] ?? ''); i++) rows.push(cells(lines[i]));
      out.push(<div key={i} className="md__table"><table><thead><tr>{head.map((c, j) => <th key={j}>{inline(c)}</th>)}</tr></thead><tbody>{rows.map((r, n) => <tr key={n}>{r.map((c, j) => <td key={j}>{inline(c)}</td>)}</tr>)}</tbody></table></div>);
    } else if (/^\s*([-*]|\d+\.)\s+/.test(l)) {
      const ordered = /^\s*\d+\./.test(l);
      const items: string[] = [];
      for (; /^\s*([-*]|\d+\.)\s+/.test(lines[i] ?? ''); i++) items.push(lines[i].replace(/^\s*([-*]|\d+\.)\s+/, ''));
      const body = items.map((t, n) => <li key={n}>{inline(t)}</li>);
      out.push(ordered ? <ol key={i}>{body}</ol> : <ul key={i}>{body}</ul>);
    } else if (/^#{1,6}\s/.test(l)) { out.push(<p key={i}><strong>{inline(l.replace(/^#+\s/, ''))}</strong></p>); i++; } else {
      const para: string[] = [];
      for (; lines[i]?.trim() && !/^\s*([-*]|\d+\.)\s+|^\s*\||^#{1,6}\s/.test(lines[i]); i++) para.push(lines[i]);
      if (!para.length) { para.push(lines[i]); i++; }
      out.push(<p key={i}>{inline(para.join('\n'))}</p>);
    }
  }
  return <div className="md">{out}</div>;
}

type Step = { id: string; label: string; status: 'running' | 'done' | 'failed' };
type Msg = { id: string; role: 'user' | 'assistant'; text: string; steps: Step[]; files?: string[] };
let seq = 0;
const stepOf = (c: Row): Step => {
  const said = String(c.description ?? '').trim() || /^\s*\/\/\s*(.+)$/m.exec(String(c.code ?? ''))?.[1];
  return { id: String(c.id), label: said || (c.state === 'writing' ? 'Writing code' : c.state === 'running' ? 'Running code' : 'Ran code'), status: c.state === 'done' ? (c.error ? 'failed' : 'done') : 'running' };
};
function toMessages(messages: Row[]): Msg[] {
  const out: Msg[] = [];
  const calls: Record<string, Row> = {};
  let now: (Msg & { calls: Row[] }) | null = null;
  for (const m of messages) {
    const content: Row[] = typeof m.content === 'string' ? [{ type: 'text', text: m.content }] : m.content ?? [];
    if (m.role === 'user') {
      for (const b of content) if (b.type === 'tool_result' && calls[b.tool_use_id]) { calls[b.tool_use_id].state = 'done'; calls[b.tool_use_id].error = b.is_error; }
      const text = content.filter(b => b.type === 'text').map(b => b.text).join('\n');
      if (text) { out.push({ id: `m${seq++}`, role: 'user', text, steps: [] }); now = null; }
      continue;
    }
    if (!now) { now = { id: `m${seq++}`, role: 'assistant', text: '', steps: [], calls: [] }; out.push(now); }
    for (const b of content) {
      if (b.type === 'text' && b.text.trim()) now.text += (now.text ? '\n\n' : '') + b.text;
      if (b.type === 'tool_use') now.calls.push((calls[b.id] = { id: b.id, description: b.input?.description ?? '', code: b.input?.code ?? '', state: 'done' }));
    }
  }
  for (const m of out as (Msg & { calls?: Row[] })[]) if (m.calls) { m.steps = m.calls.map(stepOf); delete m.calls; }
  return out;
}
function Steps({ steps }: { steps: Step[] }) {
  const [open, setOpen] = useState(false);
  if (!steps.length) return null;
  const line = (s: Step) => <span key={s.id} className={`step step--${s.status}`}>{s.status === 'running' ? <span className="step__live" /> : <Icon name={s.status === 'failed' ? 'close' : 'check'} size="sm" />}<span>{s.label}</span></span>;
  const last = steps[steps.length - 1];
  if (steps.length === 1) return <div className="steps">{line(last)}</div>;
  return (
    <div className="steps">
      <button type="button" className="steps__head" aria-expanded={open} onClick={() => setOpen(!open)}>{line(last)}<Icon name={open ? 'chevron' : 'chevronRight'} size="sm" /></button>
      {open && steps.slice(0, -1).map(line)}
    </div>
  );
}
function Composer({ placeholder, busy, autoFocus, onSend, onStop }: { placeholder: string; busy: boolean; autoFocus?: boolean; onSend: (text: string, files: File[]) => void; onStop: () => void }) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const [picked, setPicked] = useState<File[]>([]);
  const send = () => {
    if (busy || !ref.current) return;
    const text = ref.current.value.trim();
    if (!text && !picked.length) return;
    ref.current.value = '';
    ref.current.style.height = 'auto';
    const riding = picked;
    setPicked([]);
    onSend(text, riding);
  };
  return (
    <form className="card composer" onSubmit={e => { e.preventDefault(); send(); }}>
      <textarea ref={ref} className="composer__input" rows={1} placeholder={placeholder} autoFocus={autoFocus} onInput={e => { const t = e.currentTarget; t.style.height = 'auto'; t.style.height = `${t.scrollHeight}px`; }} onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }} />
      {picked.length > 0 && <div className="chips">{picked.map((f, i) => <Chip key={`${f.name}${i}`} label={f.name} onRemove={() => setPicked(picked.filter((_, j) => j !== i))} />)}</div>}
      <FilePicker inputRef={picker} multiple onFiles={f => setPicked(p => [...p, ...f].slice(0, 6))} />
      <div className="toolbar composer__bar">
        <IconButton icon="attach" label="Attach a file" onClick={() => picker.current?.click()} />
        <span className="spacer" />
        {busy ? <button type="button" className="composer__send" aria-label="Stop" onClick={onStop}><Icon name="stop" size="sm" /></button> : <button type="submit" className="composer__send" aria-label="Send"><Icon name="send" /></button>}
      </div>
    </form>
  );
}

const PAGE = 20;
function Agent({ who, agents, waiting, waitingError }: { who: Row; agents: Row[]; waiting: Row[]; waitingError?: string }) {
  const agent = agents[0];
  const { go } = useNav();
  const { lead } = useContext(ShellCtx);
  const bar = (title: string, end?: ReactNode) => <header className="chat__head">{lead}<span className="chat__title">{title}</span>{end}</header>;
  const [messages, setMessages] = useState<Msg[] | null>(null);
  const [limit, setLimit] = useState(PAGE);
  const [more, setMore] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState('');
  const [clearing, setClearing] = useState(false);
  const ctrl = useRef<AbortController | null>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const near = useRef(true);
  useEffect(() => {
    if (!agent) return;
    let live = true;
    turns(String(agent.id), limit).then(all => { if (!live) return; setMessages(toMessages([...all].reverse().flatMap(t => t.messages))); setMore(all.length === limit); }, e => { if (live) setError((e as Error).message); });
    return () => { live = false; };
  }, [agent?.id, limit]);
  useEffect(() => { const el = scroller.current; if (el && near.current) el.scrollTop = el.scrollHeight; }, [messages]);
  const hour = new Date().getHours();
  const greeting = hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
  const first = String(who.me.actor?.name || who.me.actor?.email || '').split(/[\s@.]/)[0];
  const send = async (text: string, files: File[]) => {
    if (!agent || streaming) return;
    let body = text;
    try {
      if (files.length) {
        const stored = await Promise.all(files.map(upload));
        body = `${text}\n\nAttached from the library:\n${stored.map(f => `- ${f.name} (file ${f.id})`).join('\n')}`.trim();
      }
    } catch (e) { toast((e as Error).message); return; }
    const reply: Msg = { id: `m${seq++}`, role: 'assistant', text: '', steps: [] };
    setMessages(m => [...(m ?? []), { id: `m${seq++}`, role: 'user', text, steps: [], files: files.map(f => f.name) }, reply]);
    near.current = true;
    const update = (fn: (r: Msg) => Msg) => setMessages(all => (all ?? []).map(m => (m.id === reply.id ? fn(m) : m)));
    const c = new AbortController();
    ctrl.current = c;
    setStreaming(true);
    const calls: Record<string, Row> = {};
    let segment = '';
    try {
      for await (const ev of turn({ agent: agent.id, text: body }, c.signal)) {
        if (ev.type === 'text') {
          const joint = !segment ? '\n\n' : '';
          segment += ev.text;
          update(r => ({ ...r, text: r.text ? r.text + joint + ev.text : ev.text }));
        } else if (ev.type === 'thinking') segment = '';
        else if (ev.type === 'tool') {
          segment = '';
          calls[ev.id] = { id: ev.id, description: '', code: '', state: 'writing' };
          update(r => ({ ...r, steps: [...r.steps, stepOf(calls[ev.id])] }));
        } else if (['code', 'run', 'result'].includes(ev.type) && calls[ev.id]) {
          const call = calls[ev.id];
          if (ev.code !== undefined) call.code = ev.code;
          if (ev.description) call.description = ev.description;
          if (ev.type === 'run') call.state = 'running';
          if (ev.type === 'result') { call.state = 'done'; call.error = ev.error ?? ''; }
          update(r => ({ ...r, steps: r.steps.map(s => (s.id === ev.id ? stepOf(call) : s)) }));
        } else if (ev.type === 'error') update(r => ({ ...r, text: `${r.text}${r.text ? '\n\n' : ''}*${ev.error}*` }));
      }
    } catch (e) {
      update(r => ({ ...r, text: `${r.text}${r.text ? '\n\n' : ''}*${(e as Error).name === 'AbortError' ? 'Stopped.' : (e as Error).message}*` }));
    }
    update(r => ({ ...r, steps: r.steps.map(s => (s.status === 'running' ? { ...s, status: 'failed' } : s)) }));
    ctrl.current = null;
    setStreaming(false);
    moved();
  };
  const placeholder = 'Ask about a build, a template or a vendor';
  const tasks = (
    <div className="tasks">
      {waitingError ? <Note block>{waitingError}</Note> : !waiting.length ? <span className="tasks__clear"><Icon name="check" size="sm" />Nothing is waiting on you.</span> : waiting.slice(0, 3).map(r => (
        <button key={String(r.id)} type="button" className="tasks__item" onClick={() => go(`/builds/${r.id}`)}>
          <span>{r.stage === 'review' ? 'Review and approve' : r.stage === 'draft' ? 'Send the draft' : 'Remind the vendor'}</span><span className="note">{String(r.name)}</span>
        </button>
      ))}
    </div>
  );
  if (!agent) return <div className="chat">{bar('Agent')}<div className="welcome"><div className="welcome__inner"><h1 className="welcome__title"><Icon name="sun" size="lg" />{`${greeting}, ${first}`}</h1>{tasks}<Note block>This workspace has no agent yet.</Note></div></div></div>;
  if (!messages && !error) return <div className="chat">{bar(String(agent.name ?? 'Agent'))}<div className="welcome"><Loading /></div></div>;
  if (!messages?.length) return (
    <div className="chat">{bar(String(agent.name ?? 'Agent'))}<div className="welcome">
      <div className="welcome__inner">
        <h1 className="welcome__title"><Icon name="sun" size="lg" />{`${greeting}, ${first}`}</h1>
        {tasks}
        {error && <Problem>{error}</Problem>}
        <Composer placeholder={placeholder} busy={streaming} autoFocus onSend={send} onStop={() => ctrl.current?.abort()} />
      </div>
    </div></div>
  );
  return (
    <div className="chat">
      {bar(String(agent.name ?? 'Agent'), <IconButton icon="clear" label="Clear" onClick={() => setClearing(true)} />)}
      <div className="chat__scroll" ref={scroller} onScroll={e => { const el = e.currentTarget; near.current = el.scrollHeight - el.scrollTop - el.clientHeight < 160; if (el.scrollTop < 160 && more && !streaming) { setMore(false); setLimit(l => l + PAGE); } }}>
        <div className="chat__thread">
          {messages.map(m => m.role === 'user'
            ? <div key={m.id} className="msg-user">{[m.text, ...(m.files ?? []).map(f => `Attached ${f}`)].filter(Boolean).join('\n')}</div>
            : <div key={m.id} className="msg-agent"><Steps steps={m.steps} />{m.text ? <Markdown text={m.text} /> : streaming && m === messages[messages.length - 1] && !m.steps.length && <span className="step step--running"><span className="step__live" /><span>Thinking…</span></span>}</div>)}
        </div>
      </div>
      <div className="chat__composer"><Composer placeholder="Reply…" busy={streaming} onSend={send} onStop={() => ctrl.current?.abort()} /></div>
      {clearing && <Confirm title="Clear this conversation?" note="Everything in it is gone for good." no="Cancel" yes="Clear" onYes={() => { void clearTurns(String(agent.id)).then(() => { setMessages([]); setLimit(PAGE); setClearing(false); }, e => { toast((e as Error).message); setClearing(false); }); }} onClose={() => setClearing(false)} />}
    </div>
  );
}

const ROLES = ['viewer', 'editor', 'manager', 'engineer', 'owner'].map(r => ({ value: r, label: r[0].toUpperCase() + r.slice(1) }));
function Team({ myRole }: { myRole: string }) {
  const people = useRead('member', () => list('member'), ['member', 'group']);
  const groups = useRead('group', () => list('group'), ['group']);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [open, setOpen] = useState<Row | null>(null);
  if (!people.data) return <PageState title="Team" error={people.error} />;
  const all = people.data.filter(p => p.role !== 'guest').sort((a, b) => String(a.name || a.email).localeCompare(String(b.name || b.email)));
  const groupName = (id: unknown) => groups.data?.find(g => g.id === id)?.name ?? '';
  const ready = /^\S+@\S+\.\S+$/.test(email.trim()) && !busy;
  const invite = async () => {
    if (!ready) return;
    setBusy(true); setError('');
    try { await create('member', { ...(name.trim() ? { name: name.trim() } : {}), email: email.trim().toLowerCase() }); await people.reload(); setName(''); setEmail(''); } catch (e) { setError((e as Error).message); }
    setBusy(false);
  };
  return (
    <Page title="Team" list>
      <Toolbar>
        <Field bare label="Name" value={name} placeholder="Name" onChange={setName} onEnter={invite} />
        <Field bare label="Email" type="email" value={email} placeholder="Email" onChange={setEmail} onEnter={invite} />
        <Button variant="primary" icon="mail" disabled={!ready} onClick={invite}>{busy ? 'Inviting…' : 'Invite'}</Button>
      </Toolbar>
      {error && <Problem>{error}</Problem>}
      <Table<Row> rows={all} onRow={setOpen} columns={[
        { key: 'name', label: 'Name', fill: true, render: p => String(p.name || p.email || '—') },
        { key: 'email', label: 'Email', soft: true, render: p => p.email ?? '' },
        { key: 'group', label: 'Group', soft: true, render: p => groupName(p.group) },
        { key: 'role', label: 'Role', num: true, render: p => <Tag quiet>{ROLES.find(r => r.value === p.role)?.label ?? String(p.role)}</Tag> }
      ]} />
      {open && <Member p={open} groups={groups.data ?? []} myRole={myRole} onClose={() => setOpen(null)} onDone={people.reload} />}
    </Page>
  );
}
function Member({ p, groups, myRole, onClose, onDone }: { p: Row; groups: Row[]; myRole: string; onClose: () => void; onDone: () => Promise<void> }) {
  const [name, setName] = useState(String(p.name ?? ''));
  const [role, setRole] = useState(String(p.role ?? ''));
  const [group, setGroup] = useState(String(p.group ?? ''));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const act = (write: () => Promise<unknown>) => async () => {
    setBusy(true); setError('');
    try { await write(); await onDone(); onClose(); } catch (e) { setError((e as Error).message); setBusy(false); }
  };
  const changed = name.trim() !== String(p.name ?? '') || role !== p.role || group !== String(p.group ?? '');
  const mine = ROLES.findIndex(r => r.value === myRole);
  return (
    <Dialog title={String(p.name || p.email)} onClose={onClose} foot={<>
      <Button icon="close" disabled={busy} onClick={act(() => remove('member', String(p.id)))}>Remove from the team</Button>
      <Button variant="primary" disabled={busy} onClick={changed ? act(() => patch('member', String(p.id), { name: name.trim(), role, group: group || null })) : onClose}>{changed ? 'Save' : 'Done'}</Button>
    </>}>
      {error && <Problem>{error}</Problem>}
      <Field label="Name" value={name} onChange={setName} />
      <Combo label="Role" value={role} options={ROLES.filter((_, i) => i <= mine)} onChange={setRole} />
      <Combo label="Group" placeholder="No group" value={group} options={[{ value: '', label: 'No group' }, ...groups.map(g => ({ value: String(g.id), label: String(g.name) }))]} onChange={setGroup} />
      <List items={[{ key: 'email', left: 'Email', right: String(p.email ?? '') }, { key: 'added', left: 'Added', right: show(p.created_at) }]} />
    </Dialog>
  );
}
const THEMES: { id: string; label: string; icon: IconName }[] = [{ id: 'light', label: 'Light', icon: 'sun' }, { id: 'dark', label: 'Dark', icon: 'moon' }, { id: 'system', label: 'Match system', icon: 'monitor' }];
function Settings({ who, name, theme, pick }: { who: Row; name: string; theme: string; pick: (t: string) => void }) {
  const [value, setValue] = useState(name);
  const keep = () => { const v = value.trim(); if (v && v !== name) patchWorkspace({ name: v }).then(() => toast('Saved.', 'notice'), e => toast((e as Error).message)); };
  const agent: Row | undefined = (who.agents ?? [])[0];
  return (
    <Page title="Settings">
      <Section title="Workspace"><Field label="Name" value={value} onChange={setValue} onLeave={keep} onEscape={() => setValue(name)} /></Section>
      {agent && <Section title="Agent"><Instructions id={String(agent.id)} /></Section>}
      <Section title="You"><List items={[{ key: 'n', left: 'Name', right: String(who.me.actor?.name ?? '') }, { key: 'e', left: 'Email', right: String(who.me.actor?.email ?? '') }]} /></Section>
      <Section title="Appearance"><div><Tabs value={theme} onChange={pick} tabs={THEMES} /></div></Section>
    </Page>
  );
}
function Instructions({ id }: { id: string }) {
  const [text, setText] = useState<string | null>(null);
  const [said, setSaid] = useState('');
  const held = useRef('');
  useEffect(() => { get('agent', id).then(r => { held.current = String(r.instructions ?? ''); setText(held.current); }, e => setSaid((e as Error).message)); }, [id]);
  useEffect(() => {
    if (text === null || text.trim() === held.current) return;
    const soon = setTimeout(() => { setSaid('Saving…'); patch('agent', id, { instructions: text.trim() }).then(() => { held.current = text.trim(); setSaid('Saved'); }, e => setSaid((e as Error).message)); }, 800);
    return () => clearTimeout(soon);
  }, [text, id]);
  return <><Field label="Instructions" bare lines={6} value={text ?? ''} disabled={text === null} onChange={setText} placeholder="For example: call vendors by their first name. Always say which build you mean by its number." />{said && <Note>{said}</Note>}</>;
}

function Root() {
  const [state, setState] = useState<{ who?: Row; out?: boolean; error?: string }>({});
  useEffect(() => {
    (async () => {
      let who = await boot();
      const workspaces: Row[] = who.me?.workspaces ?? [];
      const ours = workspaces.find(w => w.slug === 'lulugeorgia');
      if (ours && who.me.workspace !== ours.id) { setWorkspace(String(ours.id)); who = await boot(); } else if (ours) setWorkspace(String(ours.id));
      setState({ who });
    })().catch(e => ((e as { status?: number }).status === 401 ? setState({ out: true }) : setState({ error: (e as Error).message })));
  }, []);
  if (state.out) return <SignIn />;
  if (state.error) return <main className="gate"><Note block>{state.error}</Note></main>;
  if (!state.who) return <main className="gate"><Loading /></main>;
  const name = String(state.who.me.workspaces?.find((w: Row) => w.id === state.who!.me.workspace)?.name ?? '');
  return <Shell who={state.who} name={name} />;
}

createRoot(document.getElementById('root')!).render(
  <StrictMode><FloatingProvider>{location.pathname.startsWith('/v/') ? <VendorPortal /> : <Root />}</FloatingProvider></StrictMode>
);
