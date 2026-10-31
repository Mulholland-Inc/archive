import type { ReactNode } from 'react';
import { cx } from '../cx';
import { CircleCheck, Lightbulb, OctagonX, TriangleAlert, type LucideIcon } from 'lucide-react';
import { Icon } from './Icon';
import './Callout.css';

export type Tone = 'info' | 'success' | 'warning' | 'danger';

const glyph: Record<Tone, LucideIcon> = {
	info: Lightbulb,
	success: CircleCheck,
	warning: TriangleAlert,
	danger: OctagonX
};

/* A line the page wants read, in a tint, its mark leading it. Give it its
   own icon where one says it better than the tone's. */
export function Callout({
	tone = 'info',
	icon,
	title,
	children,
	className
}: {
	tone?: Tone;
	icon?: LucideIcon;
	title?: string;
	children: ReactNode;
	className?: string;
}) {
	return (
		<div
			className={cx('callout', `callout-${tone}`, className)}
			role={tone === 'danger' ? 'alert' : 'status'}
		>
			<Icon icon={icon ?? glyph[tone]} className="callout-glyph" size="lg" />
			<div className="callout-body">
				{title && <div className="callout-title">{title}</div>}
				<div className="callout-text">{children}</div>
			</div>
		</div>
	);
}
