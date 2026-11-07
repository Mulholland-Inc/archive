import { createContext, Fragment, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState, type ButtonHTMLAttributes, type KeyboardEvent, type ReactNode, type RefObject } from 'react';
import { createPortal } from 'react-dom';
import {
  ArrowUp, ArrowUpRight, Bed, Blinds, Box, Brush, Building, Camera, Check, ChevronDown, ChevronLeft, ChevronRight, Clock, Copy, Download, Ellipsis, EyeOff, Eraser, Fan, File, Frame, Gift, Globe, House, IdCard, Image, Lamp,
  LayoutDashboard, Lightbulb, List as ListIcon, LogOut, Mail, MessageSquare, Minus, Monitor, Moon, Palette, PanelLeft, Paperclip, Pencil, Plus, RectangleHorizontal, RectangleVertical, Search, Settings, Shield, Sofa, Square, SquareRoundCorner, Star, Sun, Table2, TriangleAlert, Upload, User, X, type LucideIcon
} from 'lucide-react';
import { fileUrl } from './api';

const icons = {
  plus: Plus, send: ArrowUp, close: X, pencil: Pencil, eyeOff: EyeOff, chevron: ChevronDown, chevronRight: ChevronRight, open: ArrowUpRight, download: Download, check: Check, alert: TriangleAlert, clock: Clock, dash: Minus,
  upload: Upload, table: Table2, camera: Camera, list: ListIcon, info: Lightbulb, box: Box, layout: LayoutDashboard, user: User, file: File, mail: Mail, globe: Globe, copy: Copy, sun: Sun, moon: Moon, monitor: Monitor,
  sidebar: PanelLeft, search: Search, more: Ellipsis, building: Building, back: ChevronLeft, lamp: Lamp, sofa: Sofa, rug: RectangleHorizontal, home: House, gift: Gift, bed: Bed, curtain: Blinds, pillow: SquareRoundCorner,
  image: Image, star: Star, brush: Brush, swatch: Palette, fan: Fan, frame: Frame, mirror: RectangleVertical, settings: Settings, badge: IdCard, signOut: LogOut, shield: Shield, attach: Paperclip, chat: MessageSquare, stop: Square, clear: Eraser
} satisfies Record<string, LucideIcon>;
export type IconName = keyof typeof icons;
export type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info';

export function Icon({ name, size = 'md' }: { name: IconName; size?: 'xs' | 'sm' | 'md' | 'lg' }) {
  const Drawn = icons[name];
  return <Drawn className={size === 'md' ? 'icon' : `icon icon--${size}`} aria-hidden="true" />;
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'quiet'; icon?: IconName; iconEnd?: IconName; pressed?: boolean };
export function Button({ variant, icon, iconEnd, pressed, children, className = '', ...rest }: ButtonProps) {
  return (
    <button type="button" className={`btn ${variant ? `btn--${variant}` : ''} ${pressed ? 'btn--on' : ''} ${className}`} aria-pressed={pressed} {...rest}>
      {icon && <Icon name={icon} />}{children}{iconEnd && <Icon name={iconEnd} />}
    </button>
  );
}
export function IconButton({ icon, label, reveal, ...rest }: ButtonHTMLAttributes<HTMLButtonElement> & { icon: IconName; label: string; reveal?: boolean }) {
  return <button type="button" className={`icon-btn ${reveal ? 'icon-btn--reveal' : ''}`} aria-label={label} title={label} {...rest}><Icon name={icon} /></button>;
}
export function CopyButton({ value, label }: { value: string; label: string }) {
  const [copied, setCopied] = useState(false);
  return <IconButton icon={copied ? 'check' : 'copy'} label={label} onClick={() => { void navigator.clipboard.writeText(value); setCopied(true); setTimeout(() => setCopied(false), 1400); }} />;
}
export const LinkButton = ({ onClick, children }: { onClick: () => void; children: ReactNode }) => <button type="button" className="link-btn" onClick={onClick}>{children}</button>;
export const Indicator = ({ icon, label, accent }: { icon: IconName; label: string; accent?: boolean }) => <span className={`indicator ${accent ? 'indicator--accent' : ''}`} title={label} aria-label={label} role="img"><Icon name={icon} size="sm" /></span>;
export const Spacer = () => <span className="spacer" />;
export const Tag = ({ tone = 'neutral', title, quiet, children }: { tone?: Tone; title?: string; quiet?: boolean; children: ReactNode }) =>
  quiet ? <span className="tag tag--quiet" title={title}>{children}</span> : <span className={`tag tag--${tone}`} title={title}><span className="tag__dot" aria-hidden="true" />{children}</span>;
export const Note = ({ block, danger, children }: { block?: boolean; danger?: boolean; children: ReactNode }) => <span className={`note ${block ? 'note--block' : ''} ${danger ? 'note--danger' : ''}`}>{children}</span>;
export const Empty = ({ children }: { children: ReactNode }) => <div className="empty">{children}</div>;
export const Toolbar = ({ wrap, children }: { wrap?: boolean; children: ReactNode }) => <div className={`toolbar ${wrap ? 'toolbar--wrap' : ''}`}>{children}</div>;
export const Lines = ({ top, sub }: { top: ReactNode; sub?: ReactNode }) => <span className="lines"><span>{top}</span>{sub && <span className="lines__sub">{sub}</span>}</span>;
export const IconLabel = ({ icon, lead, children }: { icon?: IconName; lead?: ReactNode; children: ReactNode }) => <span className="row-icon">{icon ? <Icon name={icon} /> : lead}{typeof children === 'string' ? <span>{children}</span> : children}</span>;
export const Labeled = ({ label, aside, children }: { label: string; aside?: ReactNode; children: ReactNode }) => (
  <div className="field">{aside ? <div className="field__head"><span className="field__label">{label}</span><span className="toolbar">{aside}</span></div> : <span className="field__label">{label}</span>}{children}</div>
);
export const FormRow = ({ children }: { children: ReactNode }) => <div className="form-row">{children}</div>;
export const FormGroup = ({ label, children }: { label?: string; children: ReactNode }) => <div className="form-group">{label && <span className="field__label">{label}</span>}{children}</div>;
export const FormLine = ({ words, note, wrap, children }: { words?: ReactNode; note?: ReactNode; wrap?: boolean; children?: ReactNode }) => (
  <div className={`form-line ${wrap ? 'form-line--wrap' : ''}`}>{words !== undefined && <span className="form-line__words">{words}</span>}{note && <Note>{note}</Note>}{children}</div>
);
export const FormCard = ({ children }: { children: ReactNode }) => <div className="card form-card">{children}</div>;

export function Tabs<T extends string>({ tabs, value, onChange }: { tabs: { id: T; label: string; icon?: IconName }[]; value: T; onChange: (id: T) => void }) {
  return (
    <div className="tabs" role="tablist">
      {tabs.map(t => <button key={t.id} type="button" role="tab" aria-selected={t.id === value} className={`tabs__tab ${t.id === value ? 'tabs__tab--on' : ''}`} onClick={() => onChange(t.id)}>{t.icon && <Icon name={t.icon} />}<span>{t.label}</span></button>)}
    </div>
  );
}
export function Meter({ value, label }: { value: number; label: string }) {
  const done = Math.max(0, Math.min(1, value));
  return (
    <span className={`meter ${done >= 1 ? 'meter--done' : ''}`} role="progressbar" aria-valuenow={Math.round(done * 100)} aria-valuemin={0} aria-valuemax={100}>
      <span className="meter__track"><span className="meter__fill" style={{ width: `${Math.round(done * 100)}%` }} /></span><span className="meter__label">{label}</span>
    </span>
  );
}
export const Stages = ({ stages, at }: { stages: string[]; at: number }) => (
  <span className="stages" role="img" aria-label={at < 0 ? 'Not started' : stages[at]}>
    {stages.map((s, i) => <span key={s} className={`stages__stage ${i <= at ? 'stages__stage--done' : ''} ${i === at ? 'stages__stage--now' : ''}`}><span className="stages__bar" /><span className="stages__name">{s}</span></span>)}
  </span>
);
export function Checkbox({ checked, some, label, onChange }: { checked: boolean; some?: boolean; label: string; onChange: (next: boolean) => void }) {
  return (
    <button type="button" role="checkbox" aria-checked={some && !checked ? 'mixed' : checked} aria-label={label} className={`check ${checked || some ? 'check--on' : ''}`} onClick={e => { e.stopPropagation(); e.preventDefault(); onChange(!checked); }}>
      {(checked || some) && <Icon name={checked ? 'check' : 'dash'} />}
    </button>
  );
}
export const CheckField = ({ label, checked, onChange, children }: { label: string; checked: boolean; onChange: (next: boolean) => void; children?: ReactNode }) => (
  <label className="form-check"><Checkbox label={label} checked={checked} onChange={onChange} /><span onClick={() => onChange(!checked)}>{children ?? label}</span></label>
);
export const Chip = ({ label, gone, onRemove }: { label: string; gone?: boolean; onRemove: () => void }) => (
  <button type="button" className={`chip ${gone ? 'chip--gone' : ''}`} title={`Take out ${label}`} onClick={onRemove}><span>{label}</span><Icon name="close" size="xs" /></button>
);
export function Problem({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const words = typeof children === 'string' ? children : '';
  useEffect(() => { ref.current?.scrollIntoView({ block: 'center', behavior: 'smooth' }); }, [words]);
  return <div ref={ref}><Tag tone="danger">{children}</Tag></div>;
}

export function Field({ label, bare, short, value, placeholder, type = 'text', lines, hint, autoFocus, disabled, onChange, onEnter, onFocus, onLeave, onEscape }: {
  label: string; bare?: boolean; short?: boolean; value: string; placeholder?: string; type?: 'text' | 'email' | 'date'; lines?: number; hint?: ReactNode; autoFocus?: boolean; disabled?: boolean;
  onChange: (v: string) => void; onEnter?: () => void; onFocus?: () => void; onLeave?: () => void; onEscape?: () => void;
}) {
  const key = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && onEnter) { e.preventDefault(); onEnter(); }
    else if (e.key === 'Enter' && onLeave) e.currentTarget.blur();
    else if (e.key === 'Escape' && onEscape) { onEscape(); e.currentTarget.blur(); }
  };
  const common = { className: `field__input ${lines ? 'field__input--lines' : ''}`, 'aria-label': bare ? label : undefined, value, placeholder, autoFocus, disabled };
  return (
    <label className={`field ${bare ? 'field--bare' : ''} ${short ? 'field--short' : ''}`}>
      {!bare && <span className="field__label">{label}</span>}
      {lines ? <textarea {...common} rows={lines} onChange={e => onChange(e.target.value)} /> : <input {...common} type={type} onChange={e => onChange(e.target.value)} onFocus={onFocus} onBlur={onLeave} onKeyDown={key} />}
      {hint && <span className="field__hint">{hint}</span>}
    </label>
  );
}
export function SearchField({ placeholder, value, onChange, autoFocus, onEnter }: { placeholder: string; value: string; onChange: (v: string) => void; autoFocus?: boolean; onEnter?: () => void }) {
  return <label className="search"><Icon name="search" size="sm" /><input autoFocus={autoFocus} value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)} onKeyDown={onEnter && (e => { if (e.key === 'Enter') { e.preventDefault(); onEnter(); } })} /></label>;
}

type Kind = 'list' | 'pick';
type Open = { anchor: HTMLElement; content: ReactNode; kind: Kind } | null;
const FloatCtx = createContext<{ open: (anchor: HTMLElement, content: ReactNode, kind?: Kind) => void; close: () => void; anchor: HTMLElement | null }>({ open: () => {}, close: () => {}, anchor: null });
export const useFloating = () => useContext(FloatCtx);
export function FloatingProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<Open>(null);
  const [pos, setPos] = useState({ x: -9999, y: -9999, up: false });
  const ref = useRef<HTMLDivElement>(null);
  const open = useCallback((anchor: HTMLElement, content: ReactNode, kind: Kind = 'pick') => setState({ anchor, content, kind }), []);
  const close = useCallback(() => setState(null), []);
  useLayoutEffect(() => {
    if (!state || !ref.current) return;
    const r = state.anchor.getBoundingClientRect();
    const w = ref.current.offsetWidth, h = ref.current.offsetHeight;
    const x = Math.min(Math.max(0, r.left), innerWidth - w);
    if (state.kind === 'pick') return setPos({ x, y: Math.max(12, Math.min(r.top, innerHeight - h - 12)), up: false });
    const up = r.bottom + h > innerHeight - 12;
    setPos({ x, y: up ? Math.max(12, r.top - h) : r.bottom, up });
  }, [state]);
  useEffect(() => {
    if (!state) return;
    const down = (e: MouseEvent) => { const t = e.target as Node; if (!ref.current?.contains(t) && !state.anchor.contains(t)) close(); };
    const key = (e: globalThis.KeyboardEvent) => { if (e.key === 'Escape') { e.preventDefault(); close(); } };
    const scroll = (e: Event) => { if (!ref.current?.contains(e.target as Node)) close(); };
    document.addEventListener('mousedown', down);
    document.addEventListener('keydown', key);
    document.addEventListener('scroll', scroll, true);
    return () => { document.removeEventListener('mousedown', down); document.removeEventListener('keydown', key); document.removeEventListener('scroll', scroll, true); };
  }, [state, close]);
  return (
    <FloatCtx.Provider value={{ open, close, anchor: state?.anchor ?? null }}>
      {children}
      {state && <div ref={ref} className={`floating floating--${state.kind} ${pos.up ? 'floating--up' : ''}`} role="listbox" style={{ left: pos.x, top: pos.y, width: state.kind === 'list' ? state.anchor.offsetWidth : undefined, minWidth: state.kind === 'pick' ? state.anchor.offsetWidth : undefined }}>{state.content}</div>}
    </FloatCtx.Provider>
  );
}
export function MenuItem({ icon, label, sub, checked, onClick }: { icon?: IconName; label: string; sub?: string; checked?: boolean; onClick: () => void }) {
  return <button type="button" className="menu-item" onClick={onClick}>{icon && <Icon name={icon} />}<Lines top={label} sub={sub} />{checked && <Icon name="check" />}</button>;
}
export type Option = { value: string; label: string; sub?: string; icon?: IconName };

function Searched({ label, value, options, all, pick }: { label: string; value: string; options: Option[]; all: boolean; pick: (v: string) => void }) {
  const [q, setQ] = useState('');
  const word = q.trim().toLowerCase();
  const found = options.filter(o => o.label.toLowerCase().includes(word));
  return (
    <>
      <SearchField autoFocus placeholder={`Search ${label.toLowerCase()}`} value={q} onChange={setQ} onEnter={() => { if (found.length) pick(found[0].value); }} />
      {all && !word && <MenuItem label={`Every ${label.toLowerCase()}`} checked={!value} onClick={() => pick('')} />}
      {found.map(o => <MenuItem key={o.value} label={o.label} checked={o.value === value} onClick={() => pick(o.value)} />)}
      {!found.length && <span className="menu-label">Nothing matches.</span>}
    </>
  );
}
export function Picker({ label, value, options, onChange, all = true, quiet, search }: { label: string; value: string; options: Option[]; onChange: (v: string) => void; all?: boolean; quiet?: boolean; search?: boolean }) {
  const { open, close } = useFloating();
  const now = options.find(o => o.value === value);
  const pick = (v: string) => { onChange(v); close(); };
  const menu = search ? <Searched label={label} value={value} options={options} all={all} pick={pick} /> : <>
    {all && <MenuItem label={`Every ${label.toLowerCase()}`} checked={!value} onClick={() => pick('')} />}
    {options.map(o => <MenuItem key={o.value} label={o.label} checked={o.value === value} onClick={() => pick(o.value)} />)}
  </>;
  return <Button variant={quiet ? 'quiet' : undefined} iconEnd="chevron" pressed={all && !!now} onClick={e => open(e.currentTarget, menu)}>{now ? now.label : label}</Button>;
}
export function Combo({ label, bare, value, options, placeholder, onChange, onAdd }: { label: string; bare?: boolean; value: string; options: Option[]; placeholder?: string; onChange: (v: string) => void; onAdd?: (name: string) => void }) {
  const { open, close, anchor } = useFloating();
  const box = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const [text, setText] = useState('');
  const [typed, setTyped] = useState(false);
  const now = options.find(o => o.value === value);
  const showing = !!box.current && anchor === box.current;
  const typing = options.length > 6 || !!onAdd;
  const found = (q: string) => options.filter(o => !q || o.label.toLowerCase().includes(q.toLowerCase()));
  const adds = (q: string) => !!onAdd && !!q && !options.some(o => o.label.toLowerCase() === q.toLowerCase());
  const done = () => { close(); input.current?.blur(); };
  const pick = (v: string) => { onChange(v); done(); };
  const add = (name: string) => { onAdd!(name); done(); };
  const list = (q: string) => open(box.current!, <>
    {found(q).slice(0, 80).map(o => <MenuItem key={o.value} icon={o.icon} label={o.label} sub={o.sub} checked={o.value === value} onClick={() => pick(o.value)} />)}
    {!found(q).length && !adds(q) && <span className="menu-label">Nothing matches.</span>}
    {adds(q) && <MenuItem icon="plus" label={`Add “${q}”`} onClick={() => add(q)} />}
  </>, typing ? 'list' : 'pick');
  const start = () => { setText(now?.label ?? ''); setTyped(false); list(''); if (typing) input.current?.select(); };
  const key = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Tab') return close();
    if (e.key === 'Escape') return input.current?.blur();
    if (e.key !== 'Enter') return;
    e.preventDefault();
    if (!showing) return start();
    const q = typed ? text.trim() : '';
    const first = found(q)[0];
    if (first) pick(first.value); else if (adds(q)) add(q);
  };
  return (
    <label className={`field ${bare ? 'field--bare' : ''}`}>
      {!bare && <span className="field__label">{label}</span>}
      <div ref={box} className={`field__input combo ${typing ? '' : 'combo--pick'} ${showing && typing ? 'combo--open' : ''}`}>
        {now?.icon && !(showing && typed) && <Icon name={now.icon} />}
        <input ref={input} className="combo__input" role="combobox" aria-expanded={showing} aria-label={label} readOnly={!typing} value={showing && typing ? text : now?.label ?? ''} placeholder={placeholder}
          onFocus={start} onClick={() => { if (!showing) start(); }} onChange={e => { setText(e.target.value); setTyped(true); list(e.target.value.trim()); }} onKeyDown={key} />
        <Icon name="chevron" size="sm" />
      </div>
    </label>
  );
}
export function MultiCombo({ label, values, options, placeholder, onChange }: { label: string; values: string[]; options: string[]; placeholder?: string; onChange: (values: string[]) => void }) {
  return (
    <div className="chips">
      {values.map(v => <Chip key={v} label={options.includes(v) ? v : `${v} (not an option)`} gone={!options.includes(v)} onRemove={() => onChange(values.filter(x => x !== v))} />)}
      <Combo bare label={label} placeholder={placeholder} value="" options={options.filter(o => !values.includes(o)).map(o => ({ value: o, label: o }))} onChange={v => v && onChange([...values, v])} />
    </div>
  );
}
export function IconField({ label, value, icon, options, disabled, onChange }: { label: string; value: string; icon: IconName; options: { value: string; icon: IconName }[]; disabled?: boolean; onChange: (v: string) => void }) {
  const { open, close } = useFloating();
  const grid = <div className="icon-grid">{options.map(o => <Button key={o.value} variant="quiet" icon={o.icon} pressed={o.value === value} aria-label={o.value} title={o.value} onClick={() => { close(); if (o.value !== value) onChange(o.value); }} />)}</div>;
  return (
    <div className="field field--fit">
      <span className="field__label">{label}</span>
      <button type="button" className="field__input icon-field" disabled={disabled} aria-label={`${label}: ${value || 'none'}`} title={value || undefined} onClick={e => open(e.currentTarget, grid)}><Icon name={icon} /></button>
    </div>
  );
}

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';
export function Dialog({ title, onClose, foot, wide, children }: { title: string; onClose: () => void; foot?: ReactNode; wide?: boolean; children: ReactNode }) {
  const card = useRef<HTMLElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    const before = document.activeElement as HTMLElement | null;
    const el = card.current;
    if (el && !el.contains(document.activeElement)) (el.querySelector<HTMLElement>('[autofocus], .dialog__body :is(input, textarea)') ?? el.querySelector<HTMLElement>('.dialog__foot .btn--primary:not([disabled])') ?? el.querySelector<HTMLElement>(FOCUSABLE))?.focus();
    return () => { if (before && document.contains(before)) before.focus({ preventScroll: true }); };
  }, []);
  useEffect(() => {
    const key = (e: globalThis.KeyboardEvent) => {
      if (e.defaultPrevented) return;
      const el = card.current;
      if (e.key === 'Escape') { e.preventDefault(); close.current(); return; }
      if (!el) return;
      const on = document.activeElement as HTMLElement | null;
      if (e.key === 'Tab') {
        const all = [...el.querySelectorAll<HTMLElement>(FOCUSABLE)].filter(x => x.offsetParent !== null);
        if (!all.length) return;
        const at = all.indexOf(on as HTMLElement);
        if (e.shiftKey && at <= 0) { e.preventDefault(); all[all.length - 1].focus(); } else if (!e.shiftKey && (at === all.length - 1 || at < 0)) { e.preventDefault(); all[0].focus(); }
      } else if (e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey && on?.tagName !== 'TEXTAREA' && on?.tagName !== 'BUTTON') {
        const primary = el.querySelector<HTMLButtonElement>('.dialog__foot .btn--primary:not([disabled])');
        if (primary) { e.preventDefault(); primary.click(); }
      }
    };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  }, []);
  return createPortal(
    <div className="dialog" onMouseDown={e => e.target === e.currentTarget && onClose()}>
      <section ref={card} className={`dialog__card ${wide ? 'dialog__card--wide' : ''}`} role="dialog" aria-modal="true" aria-label={title}>
        <div className="dialog__bar"><span className="dialog__title">{title}</span><IconButton icon="close" label="Close" onClick={onClose} /></div>
        <div className="dialog__body">{children}</div>
        {foot && <div className="dialog__foot">{foot}</div>}
      </section>
    </div>, document.body);
}
export const Confirm = ({ title, note, no = 'Not now', yes, busy, onYes, onClose, children }: { title: string; note: ReactNode; no?: string; yes: ReactNode; busy?: boolean; onYes: () => void; onClose: () => void; children?: ReactNode }) => (
  <Dialog title={title} onClose={onClose} foot={<><Button onClick={onClose}>{no}</Button><Button variant="primary" disabled={busy} onClick={onYes}>{yes}</Button></>}><Note block>{note}</Note>{children}</Dialog>
);
export const FilePicker = ({ inputRef, accept, multiple, onFiles }: { inputRef: RefObject<HTMLInputElement | null>; accept?: string; multiple?: boolean; onFiles: (files: File[]) => void }) => (
  <input ref={inputRef} type="file" accept={accept} multiple={multiple} hidden onChange={e => { const f = [...(e.target.files ?? [])]; e.target.value = ''; if (f.length) onFiles(f); }} />
);
export function DropZone({ title, hint, accept, onFile }: { title?: string; hint: string; accept?: string; onFile: (file: File) => void }) {
  const picker = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  return (
    <button type="button" className={`drop ${over ? 'drop--over' : ''}`} onClick={() => picker.current?.click()} onDragOver={e => { e.preventDefault(); setOver(true); }} onDragLeave={() => setOver(false)}
      onDrop={e => { e.preventDefault(); setOver(false); const f = e.dataTransfer.files[0]; if (f) onFile(f); }}>
      <Icon name="upload" /><span className="drop__words">{title && <span>{title}</span>}<span className="note">{hint}</span></span>
      <FilePicker inputRef={picker} accept={accept} onFiles={f => onFile(f[0])} />
    </button>
  );
}
export function DropRegion({ enabled, label, onFiles, children }: { enabled: boolean; label?: string; onFiles: (files: File[]) => void; children: ReactNode }) {
  const [over, setOver] = useState(false);
  return (
    <div className={`dropwrap ${over ? 'dropwrap--over' : ''}`} data-drop={label} onDragOver={e => { if (enabled && e.dataTransfer.types.includes('Files')) { e.preventDefault(); if (label) setOver(true); } }}
      onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setOver(false); }}
      onDrop={e => { if (!enabled) return; setOver(false); const files = [...e.dataTransfer.files]; if (files.length) { e.preventDefault(); onFiles(files); } }}>
      {children}
    </div>
  );
}

export type Column<R> = { key: string; label: string; num?: boolean; soft?: boolean; fill?: boolean; short?: boolean; render: (row: R) => ReactNode; sort?: (row: R) => string | number | null | undefined };
export type RowGroup<R> = { title?: string; tone?: Tone; rows: R[] };
export type Selection<R> = { ids: Set<string>; idOf: (row: R) => string; onChange: (next: Set<string>) => void };
const MARK: Record<Tone, IconName> = { success: 'check', warning: 'alert', danger: 'close', info: 'clock', neutral: 'dash' };
const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec'];
function sortKey(v: unknown): number | string | null {
  if (v == null || v === '') return null;
  if (typeof v === 'number') return v;
  const s = String(v).trim();
  const out = /^(\d+)\/(\d+)$/.exec(s);
  if (out) return Number(out[2]) ? Number(out[1]) / Number(out[2]) : 0;
  const day = /^([A-Z][a-z]{2}) (\d{1,2})$/.exec(s);
  if (day && MONTHS.includes(day[1].toLowerCase())) return MONTHS.indexOf(day[1].toLowerCase()) * 100 + Number(day[2]);
  return /^\d+(\.\d+)?$/.test(s) ? Number(s) : s.toLowerCase();
}
function compare(a: number | string | null, b: number | string | null, dir: number) {
  if (a === b) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  if (typeof a !== typeof b) return typeof a === 'number' ? -1 : 1;
  return (typeof a === 'number' ? a - (b as number) : a.localeCompare(b as string, undefined, { numeric: true })) * dir;
}
export function Table<R>({ columns, rows, groups, rowClass, onRow, select, plain }: { columns: Column<R>[]; rows?: R[]; groups?: RowGroup<R>[]; rowClass?: (row: R) => string; onRow?: (row: R) => void; select?: Selection<R>; plain?: boolean }) {
  const [by, setBy] = useState<{ key: string; dir: 1 | -1 } | null>(null);
  const given = groups ?? [{ rows: rows ?? [] }];
  const cellOf = (c: Column<R>, r: R) => sortKey(c.sort ? c.sort(r) : (v => (typeof v === 'string' || typeof v === 'number' ? v : null))(c.render(r)));
  const sortable = (c: Column<R>) => !!c.label && (!!c.sort || given.some(g => g.rows.some(r => cellOf(c, r) !== null)));
  const on = by && columns.find(c => c.key === by.key);
  const all = given.map(g => ({ ...g, keyed: on ? g.rows.map((r, k) => ({ r, k })).sort((a, b) => compare(cellOf(on, a.r), cellOf(on, b.r), by!.dir)) : g.rows.map((r, k) => ({ r, k })) }));
  const press = (key: string) => setBy(b => (b?.key !== key ? { key, dir: 1 } : b.dir === 1 ? { key, dir: -1 } : null));
  const shown = select ? all.flatMap(g => g.keyed.map(({ r }) => select.idOf(r))) : [];
  const picked = select ? shown.filter(id => select.ids.has(id)).length : 0;
  const toggle = (id: string) => { const next = new Set(select!.ids); if (next.has(id)) next.delete(id); else next.add(id); select!.onChange(next); };
  const cls = (c: Column<R>) => `${c.num ? 'num' : ''} ${c.soft ? 'is-soft' : ''} ${c.fill ? 'is-fill' : ''} ${c.short ? 'is-short' : ''}`;
  return (
    <div className={`card table-wrap ${plain ? 'table-wrap--plain' : ''}`}>
      <table className="table">
        <thead><tr>{select && <th className="is-check"><Checkbox label="Select every row" checked={picked > 0 && picked === shown.length} some={picked > 0} onChange={() => select.onChange(picked === shown.length ? new Set() : new Set(shown))} /></th>}
          {columns.map(c => (
            <th key={c.key} className={cls(c)} aria-sort={by?.key === c.key ? (by.dir === 1 ? 'ascending' : 'descending') : undefined}>
              {sortable(c) ? <button type="button" className={`th-sort ${by?.key === c.key ? 'th-sort--on' : ''} ${by?.key === c.key && by.dir === 1 ? 'th-sort--up' : ''}`} onClick={() => press(c.key)}>{c.label}<Icon name="chevron" size="xs" /></button> : c.label}
            </th>
          ))}</tr></thead>
        <tbody>
          {all.map((g, gi) => (
            <Fragment key={gi}>
              {g.title && <tr className={`table__group table__group--${g.tone ?? 'neutral'}`}><th colSpan={columns.length + (select ? 1 : 0)} scope="colgroup"><span className="table__group-name">{g.tone && <Icon name={MARK[g.tone]} size="sm" />}{g.title}</span></th></tr>}
              {g.keyed.map(({ r, k }) => (
                <tr key={k} className={`${rowClass?.(r) ?? ''} ${onRow ? 'is-link' : ''} ${select?.ids.has(select.idOf(r)) ? 'is-picked' : ''}`} onClick={onRow && (() => onRow(r))}>
                  {select && <td className="is-check"><Checkbox label="Select this row" checked={select.ids.has(select.idOf(r))} onChange={() => toggle(select.idOf(r))} /></td>}
                  {columns.map(c => <td key={c.key} className={cls(c)}>{c.render(r)}</td>)}
                </tr>
              ))}
            </Fragment>
          ))}
        </tbody>
      </table>
    </div>
  );
}
export type Item = { key: string; left: ReactNode; right?: ReactNode; below?: ReactNode; onOpen?: () => void };
export const List = ({ items }: { items: Item[] }) => (
  <div className="card list">
    {items.map(i => i.onOpen
      ? <button key={i.key} type="button" className="list__item list__item--open" onClick={i.onOpen}><span className="list__left">{i.left}</span><span className="list__end">{i.right}<Icon name="chevronRight" size="sm" /></span></button>
      : i.below ? <div key={i.key} className="list__item list__item--below"><span>{i.left}</span><span className="list__below">{i.below}</span></div>
        : <div key={i.key} className="list__item"><span className="list__left">{i.left}</span><span>{i.right}</span></div>)}
  </div>
);
export const Log = ({ lines }: { lines: { key: string; when: ReactNode; text: ReactNode }[] }) => (
  <div className="card log">{lines.map(l => <div key={l.key} className="log__line"><span className="note">{l.when}</span><span>{l.text}</span></div>)}</div>
);
export const Section = ({ title, action, children }: { title?: string; action?: ReactNode; children: ReactNode }) => {
  const name = title && <h3 className="section__title">{title}</h3>;
  return <div className="section">{action ? <div className="section__head">{name ?? <span />}<span className="toolbar">{action}</span></div> : name}{children}</div>;
};

export type Action = { key: string; label: string; icon?: IconName; primary?: boolean; disabled?: boolean; onClick: () => void };
export function Actions({ items, after }: { items: Action[]; after?: ReactNode }) {
  const { open, close } = useFloating();
  const rest = items.filter(a => !a.primary);
  const menu = (anchor: HTMLElement) => open(anchor, <>{rest.map(a => <MenuItem key={a.key} icon={a.icon} label={a.label} onClick={() => { close(); if (!a.disabled) a.onClick(); }} />)}</>);
  return (
    <div className="actions">
      {rest.length > 0 && <span className="actions__more"><IconButton icon="more" label="More" onClick={e => menu(e.currentTarget)} /></span>}
      {items.map(a => <Button key={a.key} className={a.primary ? '' : 'actions__fold'} variant={a.primary ? 'primary' : undefined} icon={a.icon} disabled={a.disabled} onClick={a.onClick}>{a.label}</Button>)}
      {after}
    </div>
  );
}

export const ShellCtx = createContext<{ lead?: ReactNode; back?: () => void }>({});
export function Page({ title, tools, aside, flush, list, wide, children }: { title: string; tools?: ReactNode; aside?: ReactNode; flush?: boolean; list?: boolean; wide?: boolean; children: ReactNode }) {
  const { lead, back } = useContext(ShellCtx);
  return (
    <div className={`page ${flush ? 'page--flush' : ''}`}>
      <div className="page__bar">
        {lead}
        {back && <IconButton icon="back" label="Back" onClick={back} />}
        <span className={`page__title ${back ? 'page__title--back' : ''}`}>{title}</span>
        <div className="page__tools">{tools}</div>
        {aside}
      </div>
      <div className={`page__body ${flush ? 'page__body--flush' : ''} ${list ? 'page__body--list' : ''} ${wide ? 'page__body--wide' : ''}`}>{children}</div>
    </div>
  );
}
export const Loading = () => <span className="loading" role="status" aria-label="Reading"><i /><i /><i /></span>;
export const PageState = ({ title, error }: { title: string; error?: string }) => <Page title={title}>{error ? <Note block>{error}</Note> : <Loading />}</Page>;

export function Pic({ file, alt = '' }: { file?: string | null; alt?: string }) {
  const [src, setSrc] = useState<string>();
  const ref = useRef<HTMLImageElement>(null);
  useEffect(() => {
    if (!file) return setSrc(undefined);
    let live = true;
    const seen = new IntersectionObserver(all => {
      if (!all.some(e => e.isIntersecting)) return;
      seen.disconnect();
      fileUrl(file).then(u => { if (live) setSrc(u); }, () => {});
    });
    seen.observe(ref.current!);
    return () => { live = false; seen.disconnect(); };
  }, [file]);
  return <img ref={ref} className="pic" src={src} alt={alt} draggable={false} />;
}

type Said = { id: number; tone: 'error' | 'notice'; text: string };
let said: Said[] = [];
let nextId = 1;
const hearers = new Set<(s: Said[]) => void>();
const tell = () => hearers.forEach(h => h(said));
const dismiss = (id: number) => { said = said.filter(s => s.id !== id); tell(); };
export function toast(text: string, tone: Said['tone'] = 'error') {
  const words = text.trim();
  if (!words || said.some(s => s.text === words && s.tone === tone)) return;
  const one = { id: nextId++, tone, text: words };
  said = [...said.slice(-3), one];
  tell();
  setTimeout(() => dismiss(one.id), tone === 'error' ? 8000 : 4000);
}
export function Toasts() {
  const [shown, setShown] = useState(said);
  useEffect(() => { hearers.add(setShown); return () => { hearers.delete(setShown); }; }, []);
  if (!shown.length) return null;
  return (
    <div className="toasts" role="status" aria-live="polite">
      {shown.map(s => (
        <div key={s.id} className={`toast toast--${s.tone}`} role={s.tone === 'error' ? 'alert' : undefined}>
          <Icon name={s.tone === 'error' ? 'alert' : 'check'} size="sm" /><span className="toast__text">{s.text}</span>
          <button type="button" className="toast__close" aria-label="Close" onClick={() => dismiss(s.id)}><Icon name="close" size="xs" /></button>
        </div>
      ))}
    </div>
  );
}

export const GONE = '\u0000gone';
export function useWrite(reread: () => Promise<unknown>, settled?: () => void, moved?: () => void) {
  const [busy, setBusy] = useState(false);
  const run = async (what: () => Promise<string | void>, tone?: 'error') => {
    setBusy(true);
    let gone = false;
    try {
      const told = await what();
      if (told === GONE) gone = true; else if (told) toast(told, tone ?? 'notice');
    } catch (e) { toast((e as Error).message); }
    moved?.();
    if (!gone) await reread();
    settled?.();
    setBusy(false);
  };
  return { busy, run };
}
