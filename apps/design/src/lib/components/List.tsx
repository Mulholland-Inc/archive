import type { ReactNode } from 'react';
import { cx } from '../cx';
import './List.css';

/* Rows with a leading slot, a title, a line under it, and a trailing slot.
   Hairlines between them, a tint on hover, nothing boxed. */
export function List({
	children,
	inset = false,
	className
}: {
	children: ReactNode;
	/** The rows carry the page's inset themselves, so the list can run edge to edge. */ inset?: boolean;
	className?: string;
}) {
	return <div className={cx('list', inset && 'list-inset', className)}>{children}</div>;
}

export function ListRow({
	leading,
	title,
	subtitle,
	trailing,
	onClick,
	wrap = false,
	className
}: {
	leading?: ReactNode;
	title: ReactNode;
	subtitle?: ReactNode;
	trailing?: ReactNode;
	onClick?: () => void;
	/** Let the words run to more than one line. */ wrap?: boolean;
	className?: string;
}) {
	const Tag = onClick ? 'button' : 'div';
	return (
		<Tag
			type={onClick ? 'button' : undefined}
			className={cx(
				'list-row',
				onClick && 'list-row-clickable chrome',
				wrap && 'list-row-wrap',
				className
			)}
			onClick={onClick}
		>
			{leading && <span className="list-leading">{leading}</span>}
			<span className="list-text">
				<span className="list-title">{title}</span>
				{subtitle && <span className="list-subtitle">{subtitle}</span>}
			</span>
			{trailing && <span className="list-trailing">{trailing}</span>}
		</Tag>
	);
}
