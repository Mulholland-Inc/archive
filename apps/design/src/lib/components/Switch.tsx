import { cx } from '../cx';
import './Switch.css';

export function Switch({
	checked,
	onChange,
	label,
	disabled,
	className
}: {
	checked: boolean;
	onChange: (checked: boolean) => void;
	label?: string;
	disabled?: boolean;
	className?: string;
}) {
	return (
		<label className={cx('switch chrome', disabled && 'switch-disabled', className)}>
			<input
				type="checkbox"
				role="switch"
				className="switch-input"
				checked={checked}
				disabled={disabled}
				onChange={(e) => onChange(e.target.checked)}
			/>
			<span className="switch-track" aria-hidden="true">
				<span className="switch-thumb" />
			</span>
			{label && <span className="switch-label">{label}</span>}
		</label>
	);
}
