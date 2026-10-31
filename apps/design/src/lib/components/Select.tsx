import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Check, ChevronDown } from 'lucide-react';
import { cx } from '../cx';
import { Anchor, Layer, useDismiss, useFlip } from '../floating';
import { Icon } from './Icon';
import './Select.css';

export interface SelectOption<T extends string> {
	value: T;
	label: string;
	hint?: string;
}

/* A custom select. The list is the menu's; the face is an input's. */
export function Select<T extends string>({
	options,
	value,
	onChange,
	placeholder = 'Choose',
	size = 'md',
	disabled,
	quiet,
	className
}: {
	options: SelectOption<T>[];
	value: T | null;
	onChange: (value: T) => void;
	placeholder?: string;
	size?: 'sm' | 'md' | 'lg';
	disabled?: boolean;
	quiet?: boolean;
	className?: string;
}) {
	const [open, setOpen] = useState(false);
	const [active, setActive] = useState(-1);
	const anchor = useRef<HTMLButtonElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const close = useCallback(() => setOpen(false), []);
	const inside = useMemo(() => [anchor, layer], []);
	useDismiss(open, close, inside);
	const current = options.find((o) => o.value === value);
	const placement = useFlip(open, anchor, 'bottom-start');

	useEffect(() => {
		if (!open) return;
		setActive(
			Math.max(
				0,
				options.findIndex((o) => o.value === value)
			)
		);
	}, [open, options, value]);

	useEffect(() => {
		if (!open) return;
		const key = (e: KeyboardEvent) => {
			if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
				e.preventDefault();
				setActive((a) =>
					e.key === 'ArrowDown'
						? (a + 1) % options.length
						: (a - 1 + options.length) % options.length
				);
			}
			if (e.key === 'Enter' && active >= 0) {
				onChange(options[active].value);
				setOpen(false);
			}
		};
		document.addEventListener('keydown', key);
		return () => document.removeEventListener('keydown', key);
	}, [open, active, options, onChange]);

	return (
		<Anchor block>
			<button
				ref={anchor}
				type="button"
				className={cx(
					'select chrome',
					`select-${size}`,
					open && 'select-open',
					!current && 'select-empty',
					quiet && 'select-quiet',
					className
				)}
				disabled={disabled}
				aria-haspopup="listbox"
				aria-expanded={open}
				onClick={() => setOpen((o) => !o)}
			>
				<span className="select-value">{current ? current.label : placeholder}</span>
				<Icon icon={ChevronDown} className="icon select-chevron" />
			</button>
			<Layer
				open={open}
				layerRef={layer}
				placement={placement}
				className="menu chrome"
				role="listbox"
				matchWidth
			>
				{options.map((o, i) => (
					<button
						key={o.value}
						type="button"
						role="option"
						aria-selected={o.value === value}
						className={cx('menu-item', i === active && 'menu-item-active')}
						onMouseEnter={() => setActive(i)}
						onClick={() => {
							onChange(o.value);
							setOpen(false);
						}}
					>
						<span className="menu-item-label">{o.label}</span>
						{o.hint && <span className="menu-item-hint">{o.hint}</span>}
						{o.value === value && <Icon icon={Check} />}
					</button>
				))}
			</Layer>
		</Anchor>
	);
}
