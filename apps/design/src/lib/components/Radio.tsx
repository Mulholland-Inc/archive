import { cx } from '../cx';
import './Radio.css';

export interface RadioOption<T extends string> {
	value: T;
	label: string;
	hint?: string;
}

/* A choice of one. Round is the shape people know a radio by; that is a
   control, not a status mark. */
export function RadioGroup<T extends string>({
	name,
	options,
	value,
	onChange,
	className
}: {
	name: string;
	options: RadioOption<T>[];
	value: T;
	onChange: (value: T) => void;
	className?: string;
}) {
	return (
		<div className={cx('radios', className)} role="radiogroup">
			{options.map((o) => (
				<label key={o.value} className="radio chrome">
					<input
						type="radio"
						className="radio-input"
						name={name}
						value={o.value}
						checked={value === o.value}
						onChange={() => onChange(o.value)}
					/>
					<span className="radio-mark" aria-hidden="true" />
					<span className="radio-text">
						<span className="radio-label">{o.label}</span>
						{o.hint && <span className="radio-hint">{o.hint}</span>}
					</span>
				</label>
			))}
		</div>
	);
}
