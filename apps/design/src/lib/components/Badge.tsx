import type { LucideIcon } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import type { Hue } from '../hue';
import './Badge.css';

/* A word in a tint. The word does the work; there is no dot. */
export function Badge({
	hue = 'gray',
	icon,
	children,
	className
}: {
	hue?: Hue;
	icon?: LucideIcon;
	children: string;
	className?: string;
}) {
	return (
		<span
			className={cx('badge chrome', className)}
			style={
				{
					'--badge-hue': `var(--hue-${hue}-text)`,
					'--badge-soft': `var(--hue-${hue}-soft)`
				} as React.CSSProperties
			}
		>
			{icon && <Icon icon={icon} size="sm" />}
			{children}
		</span>
	);
}
