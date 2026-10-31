import { Pipette } from 'lucide-react';
import { cx } from '../cx';
import { IconButton } from './Button';
import './Swatches.css';

/* A colour, drawn as a rounded square. Never a dot. */
export function Swatch({
	color,
	label,
	size = 'md',
	selected,
	onClick
}: {
	color: string;
	label?: string;
	size?: 'sm' | 'md';
	selected?: boolean;
	onClick?: () => void;
}) {
	const Tag = onClick ? 'button' : 'span';
	return (
		<Tag
			type={onClick ? 'button' : undefined}
			className={cx(
				'swatch-color chrome',
				`swatch-color-${size}`,
				selected && 'swatch-color-selected'
			)}
			style={{ background: color }}
			aria-label={label}
			title={label}
			onClick={onClick}
		/>
	);
}

/* The palette row: what is picked, its name, the eyedropper, then the
   colours on offer. */
export function Swatches({
	colors,
	value,
	onChange,
	onPick
}: {
	colors: Array<{ id: string; color: string; label: string }>;
	value: string;
	onChange: (id: string) => void;
	onPick?: () => void;
}) {
	const current = colors.find((c) => c.id === value);
	return (
		<div className="swatches chrome">
			{current && (
				<span className="swatches-current">
					<Swatch color={current.color} label={current.label} />
					<span className="swatches-name">{current.label}</span>
				</span>
			)}
			{onPick && <IconButton icon={Pipette} label="Pick a colour" size="sm" onClick={onPick} />}
			<span className="swatches-divider" />
			<span className="swatches-list">
				{colors.map((c) => (
					<Swatch
						key={c.id}
						color={c.color}
						label={c.label}
						selected={c.id === value}
						onClick={() => onChange(c.id)}
					/>
				))}
			</span>
		</div>
	);
}
