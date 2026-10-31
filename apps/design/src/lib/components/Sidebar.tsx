import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { ChevronsUpDown, Settings } from 'lucide-react';
import { cx } from '../cx';
import type { Hue } from '../hue';
import { Avatar } from './Avatar';
import { Menu, type MenuEntry } from './Menu';
import { IconButton } from './Button';
import { Emoji } from './Emoji';
import { Icon } from './Icon';
import { Tooltip } from './Tooltip';
import './Sidebar.css';

export type { Hue };

export interface SidebarItem {
	id: string;
	label: string;
	/** The picture on the row: the emoji a thing someone made wears, or a
	    fixed icon where the row is part of the interface. */
	emoji?: string;
	icon?: LucideIcon;
	/** Something waits here. */
	marked?: boolean;
	/** A shortcut, in the quiet tone at the end of the row. */
	hint?: string;
}

export interface SidebarSection {
	id: string;
	/** Sections with a name get it in the quiet tone above their rows;
	    the first section of a sidebar usually has none. */
	label?: string;
	items: SidebarItem[];
}

/* One row of the sidebar: a mark, a name, and what it has to say. The
   same row whether it scrolls with the rest or is pinned above them. */
function Row({
	item,
	active,
	onSelect
}: {
	item: SidebarItem;
	active: boolean;
	onSelect: (id: string) => void;
}) {
	return (
		<a
			href={`#${item.id}`}
			className={cx('sidebar-item', active && 'sidebar-item-active')}
			aria-current={active ? 'page' : undefined}
			onClick={(e) => {
				e.preventDefault();
				onSelect(item.id);
			}}
		>
			{item.emoji ? (
				<span className="sidebar-glyph">
					<Emoji>{item.emoji}</Emoji>
				</span>
			) : item.icon ? (
				<span className="sidebar-glyph">
					<Icon icon={item.icon} />
				</span>
			) : (
				<span className="sidebar-glyph" />
			)}
			<span className="sidebar-item-label">{item.label}</span>
			{item.hint && <span className="sidebar-item-hint">{item.hint}</span>}
			{item.marked && <span className="sidebar-item-mark" aria-label="Something waits" />}
		</a>
	);
}

/* The sidebar every app wears on the left. The workspace at the top,
   then sections of rows, each row a mark and a name, the open one on a
   tint. At the foot you, and the plan. Nothing is outlined; a tone shift
   is enough.

   Pinned rows sit above the rest and stay there: the sections scroll and
   they do not, which is how search stays reachable from the bottom of a
   long rail. */
export function Sidebar({
	sections,
	pinned,
	active,
	onSelect,
	header,
	footer,
	className
}: {
	sections: SidebarSection[];
	pinned?: SidebarItem[];
	active: string;
	onSelect: (id: string) => void;
	header?: ReactNode;
	footer?: ReactNode;
	className?: string;
}) {
	return (
		<nav className={cx('sidebar chrome', className)} data-tauri-drag-region>
			{header && <div className="sidebar-head">{header}</div>}
			{pinned && pinned.length > 0 && (
				<div className="sidebar-pinned">
					{pinned.map((it) => (
						<Row key={it.id} item={it} active={it.id === active} onSelect={onSelect} />
					))}
				</div>
			)}
			<div className="sidebar-body">
				{sections.map((s) => (
					<div key={s.id} className="sidebar-section">
						{s.label && <div className="sidebar-heading">{s.label}</div>}
						{s.items.map((it) => (
							<Row key={it.id} item={it} active={it.id === active} onSelect={onSelect} />
						))}
					</div>
				))}
			</div>
			{footer && <div className="sidebar-foot">{footer}</div>}
		</nav>
	);
}

/* Whose desk this is: the workspace's mark and name, and a hint that it
   can be switched. It is a button, so a menu can hang off it. */
export const Workspace = forwardRef<
	HTMLButtonElement,
	{ name: string; emoji?: string } & ButtonHTMLAttributes<HTMLButtonElement>
>(function Workspace({ name, emoji, className, ...rest }, ref) {
	return (
		<button ref={ref} type="button" className={cx('workspace chrome', className)} {...rest}>
			{emoji && <Emoji>{emoji}</Emoji>}
			<span className="workspace-name">{name}</span>
			<Icon icon={ChevronsUpDown} size="sm" className="workspace-chevron" />
		</button>
	);
});

/* The head of the sidebar: settings, and the workspace before it where
   there is more than one to be in. */
export function SidebarHead({
	workspace,
	onSettings
}: {
	workspace?: ReactNode;
	onSettings?: () => void;
}) {
	return (
		<>
			{workspace ?? <span className="sidebar-head-room" />}
			{onSettings && (
				<Tooltip label="Settings" placement="bottom">
					<IconButton icon={Settings} label="Settings" onClick={onSettings} />
				</Tooltip>
			)}
		</>
	);
}

/* The foot of the sidebar: you, the plan's usage and settings beside you. */
export function SidebarFoot({
	usage,
	user,
	onUser,
	menu,
	onMenu,
	onSettings
}: {
	usage?: ReactNode;
	user: string;
	onUser?: () => void;
	/** What pressing the person opens, where it opens something. */ menu?: MenuEntry[];
	onMenu?: (id: string) => void;
	onSettings?: () => void;
}) {
	const you = (
		<button type="button" className="sidebar-user chrome" aria-label={user} onClick={onUser}>
			<Avatar name={user} size="sm" />
			<span className="sidebar-user-name">{user}</span>
		</button>
	);
	return (
		<>
			{menu && onMenu ? (
				<Menu trigger={you} items={menu} onSelect={onMenu} placement="top-start" />
			) : (
				you
			)}
			{usage}
			{onSettings && (
				<Tooltip label="Settings" placement="top">
					<IconButton icon={Settings} label="Settings" onClick={onSettings} />
				</Tooltip>
			)}
		</>
	);
}
