import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Empty.css';

/* Nothing here yet, said plainly, with the one thing to do about it. */
export function Empty({
	icon,
	title,
	children,
	action,
	className
}: {
	/** A glyph above the title, where one says something. */ icon?: LucideIcon;
	title: string;
	children?: ReactNode;
	action?: ReactNode;
	className?: string;
}) {
	return (
		<div className={cx('empty', className)}>
			{icon && <Icon icon={icon} size="lg" className="icon icon-lg empty-glyph" />}
			<div className="empty-title">{title}</div>
			{children && <div className="empty-text">{children}</div>}
			{action && <div className="empty-action">{action}</div>}
		</div>
	);
}
