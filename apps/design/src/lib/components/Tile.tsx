import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { ArrowRight } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Tile.css';

export type TileHue = 'red' | 'navy' | 'green' | 'sky' | 'amber' | 'purple' | 'teal';

/* A big coloured block to start from: an icon, a title, one verb, and
   room on the right for the art. */
export function Tile({
	hue = 'red',
	icon,
	title,
	action = 'Browse',
	onClick,
	art,
	className
}: {
	hue?: TileHue;
	icon?: LucideIcon;
	title: string;
	action?: string;
	onClick?: () => void;
	art?: ReactNode;
	className?: string;
}) {
	return (
		<button type="button" className={cx('tile chrome', `tile-${hue}`, className)} onClick={onClick}>
			<span className="tile-text">
				{icon && <Icon icon={icon} size="lg" />}
				<span className="tile-title">{title}</span>
				<span className="tile-action">
					<span>{action}</span>
					<Icon icon={ArrowRight} size="sm" />
				</span>
			</span>
			{art && <span className="tile-art">{art}</span>}
		</button>
	);
}
