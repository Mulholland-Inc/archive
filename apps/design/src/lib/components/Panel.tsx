import type { ReactNode } from 'react';
import { X } from 'lucide-react';
import { cx } from '../cx';
import { IconButton } from './Button';
import './Panel.css';

export type PanelTone = 'ink' | 'light';

/* A floating surface. Ink is the house tone: a dark card on a coloured
   canvas that re-themes every component inside it, so a select or a
   composer dropped into a panel needs nothing of its own. */
export function Panel({
	tone = 'ink',
	title,
	leading,
	actions,
	onClose,
	children,
	footer,
	className
}: {
	tone?: PanelTone;
	title?: string;
	leading?: ReactNode;
	actions?: ReactNode;
	onClose?: () => void;
	children: ReactNode;
	footer?: ReactNode;
	className?: string;
}) {
	return (
		<section className={cx('panel', `panel-${tone}`, className)}>
			{(title || onClose || actions) && (
				<header className="panel-head chrome">
					{leading}
					{title && <span className="panel-title">{title}</span>}
					<span className="panel-actions">
						{actions}
						{onClose && <IconButton icon={X} label="Close" size="sm" onClick={onClose} />}
					</span>
				</header>
			)}
			<div className="panel-body">{children}</div>
			{footer && <div className="panel-foot">{footer}</div>}
		</section>
	);
}
