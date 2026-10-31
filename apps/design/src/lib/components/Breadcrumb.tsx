import { ChevronRight, type LucideIcon as IconGlyph } from 'lucide-react';
import { cx } from '../cx';
import { Emoji } from './Emoji';
import { Icon } from './Icon';
import './Breadcrumb.css';

export interface Crumb {
	label: string;
	/** The page's own emoji, before its name — a thing someone made wears
	    one of those. */
	emoji?: string;
	/** A fixed icon instead, where the crumb is an area of the interface
	    rather than a thing someone made. */
	icon?: IconGlyph;
	href?: string;
	onClick?: () => void;
}

/* Where the page sits. The last crumb is the page and reads as the title. */
export function Breadcrumb({ items, className }: { items: Crumb[]; className?: string }) {
	return (
		<nav className={cx('crumbs chrome', className)} aria-label="Breadcrumb">
			{items.map((c, i) => {
				const last = i === items.length - 1;
				return (
					<span key={i} className="crumb-pair">
						{last ? (
							<span className="crumb crumb-current" aria-current="page">
								{c.emoji ? (
									<Emoji size="sm">{c.emoji}</Emoji>
								) : (
									c.icon && <Icon icon={c.icon} size="sm" />
								)}
								<span>{c.label}</span>
							</span>
						) : (
							<a className="crumb" href={c.href} onClick={c.onClick}>
								{c.emoji ? (
									<Emoji size="sm">{c.emoji}</Emoji>
								) : (
									c.icon && <Icon icon={c.icon} size="sm" />
								)}
								<span>{c.label}</span>
							</a>
						)}
						{!last && <Icon icon={ChevronRight} size="sm" className="icon icon-sm crumb-sep" />}
					</span>
				);
			})}
		</nav>
	);
}
