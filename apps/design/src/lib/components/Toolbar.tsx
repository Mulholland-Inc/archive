import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { ChevronDown } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Toolbar.css';

/* A floating row of tools on the canvas. Ink, whatever the page is. */
export function Toolbar({ children, className }: { children: ReactNode; className?: string }) {
	return (
		<div className={cx('toolbar panel-ink chrome', className)} role="toolbar">
			{children}
		</div>
	);
}

export function Tool({
	icon,
	label,
	active,
	menu,
	onClick
}: {
	icon: LucideIcon;
	label: string;
	active?: boolean;
	menu?: boolean;
	onClick?: () => void;
}) {
	return (
		<button
			type="button"
			className={cx('tool', active && 'tool-active')}
			aria-label={label}
			aria-pressed={active}
			onClick={onClick}
		>
			<Icon icon={icon} />
			{menu && <Icon icon={ChevronDown} size="sm" />}
		</button>
	);
}

export function ToolDivider() {
	return <span className="tool-divider" role="separator" />;
}

/* A word, a value, and a control: "Color, E34927". */
export function ToolField({ label, children }: { label: string; children: ReactNode }) {
	return (
		<span className="tool-field">
			<span className="tool-field-label">{label}</span>
			{children}
		</span>
	);
}

export function ToolValue({ children, leading }: { children: ReactNode; leading?: ReactNode }) {
	return (
		<span className="tool-value">
			{leading}
			<span>{children}</span>
		</span>
	);
}
