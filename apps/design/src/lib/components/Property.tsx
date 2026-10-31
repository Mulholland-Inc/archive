import { cx } from '../cx';
import './Property.css';

/* A label on the left, a control on the right: one line of an inspector. */
export function Property({
	label,
	children,
	stacked = false
}: {
	label: string;
	children: React.ReactNode;
	stacked?: boolean;
}) {
	return (
		<div className={cx('property', stacked && 'property-stacked')}>
			<span className="property-label chrome">{label}</span>
			<span className="property-control">{children}</span>
		</div>
	);
}
