import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Search } from 'lucide-react';
import { cx } from '../cx';
import { Anchor, Layer, useDismiss } from '../floating';
import { Breadcrumb, type Crumb } from './Breadcrumb';
import { Dialog } from './Dialog';
import { Icon } from './Icon';
import { Input } from './Input';
import { Split } from './Split';
import './Shell.css';

/* The chrome every app wears: a header that says where you are and
   what you can do here, a command palette on the keyboard, and the
   shell that lays sidebar, the agent's column and the work. */

/* Where you are, and what you can do here. */
export function Header({
	title,
	crumbs,
	actions,
	className
}: {
	title?: ReactNode;
	crumbs?: ReactNode;
	actions?: ReactNode;
	className?: string;
}) {
	return (
		<header className={cx('header chrome', className)} data-tauri-drag-region>
			<div className="header-where">{crumbs ?? <span className="header-title">{title}</span>}</div>
			{actions && <div className="header-actions">{actions}</div>}
		</header>
	);
}

/* Sidebar, the agent's column, then the work. The agent is always
   there on the left; what it opens, or you open, lands on the right.
   Whatever is not given is not laid out. */
export function Shell({
	sidebar,
	chat,
	crumbs,
	actions,
	children,
	className
}: {
	sidebar: ReactNode;
	chat?: ReactNode;
	/** Where you are, across the top of the work, as Notion's bar has it. */ crumbs?: Crumb[];
	actions?: ReactNode;
	children: ReactNode;
	className?: string;
}) {
	const body = (
		<div className="shell-body">
			<main className="shell-main">
				{crumbs && <Header crumbs={<Breadcrumb items={crumbs} />} actions={actions} />}
				{children}
			</main>
		</div>
	);
	return (
		<div className={cx('shell', className)}>
			{sidebar}
			{chat ? (
				<Split id="chat" width={440} min={220} max={900}>
					<section className="shell-chat">{chat}</section>
					{body}
				</Split>
			) : (
				body
			)}
		</div>
	);
}

export interface CommandItem {
	id: string;
	label: string;
	hint?: string;
	icon?: LucideIcon;
	group?: string;
}

/* Everything, from the keyboard, as Notion's search: a bare line to
   type in, then rows with their mark. Cmd K opens it; type, arrow,
   Enter. */
export function Commands({
	open,
	onClose,
	items,
	onRun,
	placeholder = 'Type a command or search'
}: {
	open: boolean;
	onClose: () => void;
	items: CommandItem[];
	onRun: (id: string) => void;
	placeholder?: string;
}) {
	const [q, setQ] = useState('');
	const [active, setActive] = useState(0);
	const field = useRef<HTMLInputElement | null>(null);
	const hits = useMemo(() => {
		const s = q.trim().toLowerCase();
		return s
			? items.filter(
					(i) =>
						i.label.toLowerCase().includes(s) ||
						i.hint?.toLowerCase().includes(s) ||
						i.group?.toLowerCase().includes(s)
				)
			: items;
	}, [q, items]);
	useEffect(() => {
		if (open) {
			setQ('');
			setActive(0);
			window.setTimeout(() => field.current?.focus(), 0);
		}
	}, [open]);
	useEffect(() => setActive(0), [q]);
	const groups = useMemo(() => Array.from(new Set(hits.map((h) => h.group ?? ''))), [hits]);
	let index = -1;
	return (
		<Dialog open={open} onClose={onClose} title="Command" bare>
			<div className="command">
				<Input
					ref={field}
					icon={Search}
					size="lg"
					value={q}
					placeholder={placeholder}
					onChange={(e) => setQ(e.target.value)}
					onKeyDown={(e) => {
						if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
							e.preventDefault();
							setActive((a) =>
								hits.length
									? e.key === 'ArrowDown'
										? (a + 1) % hits.length
										: (a - 1 + hits.length) % hits.length
									: 0
							);
						}
						if (e.key === 'Enter' && hits[active]) {
							onRun(hits[active].id);
							onClose();
						}
					}}
					className="command-field"
					quiet
				/>
				<div className="command-list" role="listbox">
					{groups.map((g) => (
						<div key={g} className="command-group">
							{g && <div className="command-heading chrome">{g}</div>}
							{hits
								.filter((h) => (h.group ?? '') === g)
								.map((h) => {
									index += 1;
									const i = index;
									return (
										<button
											key={h.id}
											type="button"
											role="option"
											aria-selected={i === active}
											className={cx('command-item chrome', i === active && 'command-item-active')}
											onMouseEnter={() => setActive(i)}
											onClick={() => {
												onRun(h.id);
												onClose();
											}}
										>
											{h.icon && <Icon icon={h.icon} />}
											<span className="command-label">{h.label}</span>
											{h.hint && <span className="command-hint">{h.hint}</span>}
										</button>
									);
								})}
						</div>
					))}
					{hits.length === 0 && <div className="command-none">Nothing matches.</div>}
				</div>
			</div>
		</Dialog>
	);
}
