import { cloneElement, useEffect, useRef, useState, type ReactElement } from 'react';
import { Anchor, Layer, useFlip, type Placement } from '../floating';
import './Tooltip.css';

/* A word or two on hover, after a beat. Never the only place a label lives. */
export function Tooltip({
	label,
	children,
	placement = 'top'
}: {
	label: string;
	children: ReactElement<Record<string, unknown>>;
	placement?: Placement;
}) {
	const [open, setOpen] = useState(false);
	const anchor = useRef<HTMLElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const timer = useRef<number | undefined>(undefined);
	const show = () => {
		window.clearTimeout(timer.current);
		const delay =
			parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--tooltip-delay')) ||
			0;
		timer.current = window.setTimeout(() => setOpen(true), delay);
	};
	const hide = () => {
		window.clearTimeout(timer.current);
		setOpen(false);
	};
	useEffect(() => () => window.clearTimeout(timer.current), []);
	const side = useFlip(open, anchor, placement);
	return (
		<Anchor>
			{cloneElement(children, {
				ref: anchor,
				onMouseEnter: show,
				onMouseLeave: hide,
				onFocus: show,
				onBlur: hide
			})}
			<Layer
				open={open}
				layerRef={layer}
				placement={side}
				className="tooltip chrome"
				role="tooltip"
			>
				{label}
			</Layer>
		</Anchor>
	);
}
