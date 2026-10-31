import type { ReactNode } from 'react';
import { cx } from '../cx';
import './Field.css';

/* A label, the control, and one line under it: the hint, or the error when
   there is one. */
export function Field({
	label,
	hint,
	error,
	children,
	className
}: {
	label: string;
	hint?: string;
	error?: string;
	children: ReactNode;
	className?: string;
}) {
	return (
		<label className={cx('field', className)}>
			<span className="field-label chrome">{label}</span>
			{children}
			{(error || hint) && (
				<span className={cx('field-note', error && 'field-error')}>{error || hint}</span>
			)}
		</label>
	);
}
