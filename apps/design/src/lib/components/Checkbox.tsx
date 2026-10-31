import { forwardRef, type InputHTMLAttributes } from 'react';
import { cx } from '../cx';
import './Checkbox.css';

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
	label?: string;
	indeterminate?: boolean;
}

/* The box is drawn, the check is a stroke that draws itself in. */
export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox(
	{ label, indeterminate, className, ...rest },
	ref
) {
	return (
		<label className={cx('check chrome', rest.disabled && 'check-disabled', className)}>
			<input ref={ref} type="checkbox" className="check-input" {...rest} />
			<span className={cx('check-box', indeterminate && 'check-mixed')} aria-hidden="true">
				<svg
					viewBox="0 0 16 16"
					fill="none"
					stroke="currentColor"
					strokeWidth="2"
					strokeLinecap="round"
					strokeLinejoin="round"
				>
					{indeterminate ? (
						<path className="check-mark" d="M4 8h8" />
					) : (
						<path className="check-mark" d="M3.5 8.5l3 3 6-7" />
					)}
				</svg>
			</span>
			{label && <span className="check-label">{label}</span>}
		</label>
	);
});
