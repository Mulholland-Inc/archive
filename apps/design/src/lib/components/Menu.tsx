import {
	cloneElement,
	useCallback,
	useEffect,
	useMemo,
	useRef,
	useState,
	type ReactElement
} from 'react';
import type { LucideIcon } from 'lucide-react';
import { Check, ChevronRight } from 'lucide-react';
import { cx } from '../cx';
import { Anchor, Layer, useDismiss, useFlip, type Placement } from '../floating';
import { Icon } from './Icon';
import './Menu.css';

export interface MenuItem {
	id: string;
	label: string;
	icon?: LucideIcon;
	hint?: string;
	/** The one in force, where the menu is a choice. */
	checked?: boolean;
	danger?: boolean;
	disabled?: boolean;
	/** A submenu, opened to the side; the row itself is not a verb. */
	items?: MenuEntry[];
}

export type MenuEntry = MenuItem | { divider: true } | { heading: string };

function isItem(e: MenuEntry): e is MenuItem {
	return 'id' in e;
}

/* A list of verbs under a trigger. Arrow keys walk it, Enter runs one.
   A row with items opens them beside it; with `stay`, choosing keeps the
   menu open so several things can be set in one visit. */
export function Menu({
	trigger,
	items,
	onSelect,
	placement = 'bottom-start',
	stay = false
}: {
	trigger: ReactElement<{ onClick?: () => void; ref?: unknown }>;
	items: MenuEntry[];
	onSelect: (id: string) => void;
	placement?: Placement;
	stay?: boolean;
}) {
	const [open, setOpen] = useState(false);
	const anchor = useRef<HTMLElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const close = useCallback(() => setOpen(false), []);
	const inside = useMemo(() => [anchor, layer], []);
	useDismiss(open, close, inside);
	const side = useFlip(open, anchor, placement);
	const pick = (id: string) => {
		onSelect(id);
		if (!stay) setOpen(false);
	};
	return (
		<Anchor>
			{cloneElement(trigger, { ref: anchor, onClick: () => setOpen((o) => !o) })}
			<Layer open={open} layerRef={layer} placement={side} className="menu chrome" role="menu">
				<div
					className={cx(
						'menu-list',
						!items.some((e) => isItem(e) && e.items) && 'menu-list-scroll'
					)}
				>
					<Entries items={items} open={open} onPick={pick} />
				</div>
			</Layer>
		</Anchor>
	);
}

/* One level of the menu: the rows, the arrow keys, and whichever
   submenu is open beside its row. */
function Entries({
	items,
	open,
	onPick
}: {
	items: MenuEntry[];
	open: boolean;
	onPick: (id: string) => void;
}) {
	const [active, setActive] = useState(-1);
	const [sub, setSub] = useState(-1);
	const enabled = items.map((e, i) => (isItem(e) && !e.disabled ? i : -1)).filter((i) => i >= 0);

	useEffect(() => {
		if (!open) {
			setActive(-1);
			setSub(-1);
		}
	}, [open]);

	useEffect(() => {
		if (!open || sub >= 0) return;
		const key = (e: KeyboardEvent) => {
			if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
				e.preventDefault();
				const pos = enabled.indexOf(active);
				const next =
					e.key === 'ArrowDown'
						? enabled[(pos + 1) % enabled.length]
						: enabled[(pos - 1 + enabled.length) % enabled.length];
				setActive(next);
			}
			if ((e.key === 'Enter' || e.key === 'ArrowRight') && active >= 0) {
				const it = items[active];
				if (!isItem(it)) return;
				if (it.items) setSub(active);
				else if (e.key === 'Enter') onPick(it.id);
			}
		};
		document.addEventListener('keydown', key);
		return () => document.removeEventListener('keydown', key);
	}, [open, active, sub, enabled, items, onPick]);

	return (
		<>
			{items.map((e, i) => {
				if ('divider' in e) return <div key={i} className="menu-divider" role="separator" />;
				if ('heading' in e)
					return (
						<div key={i} className="menu-heading">
							{e.heading}
						</div>
					);
				const row = (
					<button
						type="button"
						role="menuitem"
						className={cx(
							'menu-item',
							e.danger && 'menu-item-danger',
							e.checked && 'menu-item-checked',
							(i === active || i === sub) && 'menu-item-active'
						)}
						disabled={e.disabled}
						aria-haspopup={e.items ? 'menu' : undefined}
						aria-expanded={e.items ? i === sub : undefined}
						onMouseEnter={() => {
							setActive(i);
							if (e.items) setSub(i);
							else setSub(-1);
						}}
						onClick={() => {
							if (e.items) setSub((s) => (s === i ? -1 : i));
							else onPick(e.id);
						}}
					>
						{e.icon && <Icon icon={e.icon} size="sm" />}
						<span className="menu-item-label">{e.label}</span>
						{e.hint && <span className="menu-item-hint">{e.hint}</span>}
						{e.checked && <Icon icon={Check} size="sm" className="icon icon-sm menu-item-check" />}
						{e.items && (
							<Icon icon={ChevronRight} size="sm" className="icon icon-sm menu-item-more" />
						)}
					</button>
				);
				if (!e.items) return <span key={e.id}>{row}</span>;
				return (
					<Submenu
						key={e.id}
						row={row}
						items={e.items}
						open={open && i === sub}
						onPick={onPick}
						onLeave={() => setSub((s) => (s === i ? -1 : s))}
					/>
				);
			})}
		</>
	);
}

function Submenu({
	row,
	items,
	open,
	onPick,
	onLeave
}: {
	row: ReactElement;
	items: MenuEntry[];
	open: boolean;
	onPick: (id: string) => void;
	onLeave: () => void;
}) {
	const layer = useRef<HTMLDivElement>(null);
	const at = useRef<HTMLSpanElement | null>(null);
	const side = useFlip(open, at, 'right-start');
	const leaving = useRef<number | undefined>(undefined);
	const hold = () => window.clearTimeout(leaving.current);
	const leave = () => {
		hold();
		leaving.current = window.setTimeout(onLeave, 240);
	};
	return (
		<Anchor block>
			<span ref={at} className="menu-sub-row" onMouseEnter={hold} onMouseLeave={leave}>
				{row}
			</span>
			<Layer
				open={open}
				layerRef={layer}
				placement={side}
				className="menu menu-sub chrome"
				role="menu"
			>
				<span
					className={cx(
						'menu-sub-body menu-list',
						!items.some((e) => isItem(e) && e.items) && 'menu-list-scroll'
					)}
					onMouseEnter={hold}
					onMouseLeave={leave}
				>
					<Entries items={items} open={open} onPick={onPick} />
				</span>
			</Layer>
		</Anchor>
	);
}
