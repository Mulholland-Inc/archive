import { useEffect, useState, type ReactNode, type RefObject } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import './floating.css';

/* One floating layer for menus, selects, popovers and tooltips. It is not
   a portal: the layer lives in the flow next to its trigger, positioned by
   CSS off a relative wrapper, so it scrolls with the page in the same
   frame like anything else on it. */

export type Placement =
	| 'bottom-start'
	| 'bottom'
	| 'bottom-end'
	| 'top-start'
	| 'top'
	| 'top-end'
	| 'right'
	| 'right-start'
	| 'right-end'
	| 'left'
	| 'left-start'
	| 'left-end';

export function useDismiss(
	open: boolean,
	onClose: () => void,
	inside: Array<RefObject<HTMLElement | null>>
) {
	useEffect(() => {
		if (!open) return;
		const down = (e: PointerEvent) => {
			const t = e.target as Node;
			if (inside.some((r) => r.current?.contains(t))) return;
			onClose();
		};
		const key = (e: KeyboardEvent) => {
			if (e.key === 'Escape') onClose();
		};
		document.addEventListener('pointerdown', down);
		document.addEventListener('keydown', key);
		return () => {
			document.removeEventListener('pointerdown', down);
			document.removeEventListener('keydown', key);
		};
	}, [open, onClose, inside]);
}

/* Which way a layer opens: the preferred way unless the anchor sits too
   near an edge of the window, then the other way, up or down and left
   or right. Read once when it opens, never on scroll. */
export function useFlip(
	open: boolean,
	anchor: RefObject<HTMLElement | null>,
	preferred: Placement
): Placement {
	const [placement, setPlacement] = useState<Placement>(preferred);
	useEffect(() => {
		if (!open) return;
		const el = anchor.current;
		if (!el) return;
		const r = el.getBoundingClientRect();
		const need =
			parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--layer-room')) ||
			240;
		const below = window.innerHeight - r.bottom;
		const above = r.top;
		const rightward = window.innerWidth - r.left;
		const leftward = r.right;
		const [side, align = 'center'] = preferred.split('-') as [string, string | undefined];
		if (side === 'right' || side === 'left') {
			// Beside the anchor: the side with room. Asked for no alignment
			// it stays level with the anchor's middle, which is what a short
			// layer beside a control wants; an alignment given hangs it from
			// the anchor's top unless that would run off the foot.
			const s =
				side === 'right'
					? window.innerWidth - r.right < need && r.left > need
						? 'left'
						: 'right'
					: r.left < need && window.innerWidth - r.right > need
						? 'right'
						: 'left';
			if (align === 'center') {
				setPlacement(s as Placement);
				return;
			}
			const a = window.innerHeight - r.top < need && r.bottom > need ? 'end' : 'start';
			setPlacement(`${s}-${a}` as Placement);
			return;
		}
		const v =
			side === 'bottom'
				? below < need && above > below
					? 'top'
					: 'bottom'
				: above < need && below > above
					? 'bottom'
					: 'top';
		let a = align;
		if (align === 'start' && rightward < need && leftward > need) a = 'end';
		else if (align === 'end' && leftward < need && rightward > need) a = 'start';
		else if (align === 'center') {
			const mid = r.left + r.width / 2;
			if (mid < need / 2) a = 'start';
			else if (window.innerWidth - mid < need / 2) a = 'end';
		}
		setPlacement((a === 'center' ? v : `${v}-${a}`) as Placement);
	}, [open, anchor, preferred]);
	return placement;
}

const rise = {
	initial: { opacity: 0, y: -4, scale: 0.98 },
	animate: { opacity: 1, y: 0, scale: 1 },
	exit: { opacity: 0, y: -4, scale: 0.98 }
};

const settle = {
	initial: { opacity: 0, y: 4, scale: 0.98 },
	animate: { opacity: 1, y: 0, scale: 1 },
	exit: { opacity: 0, y: 4, scale: 0.98 }
};

/* The wrapper the trigger and its layer share. */
export function Anchor({
	children,
	className,
	anchorRef,
	block = false
}: {
	children: ReactNode;
	className?: string;
	anchorRef?: RefObject<HTMLSpanElement>;
	block?: boolean;
}) {
	return (
		<span
			ref={anchorRef}
			className={['anchor', block && 'anchor-block', className].filter(Boolean).join(' ')}
		>
			{children}
		</span>
	);
}

export function Layer({
	open,
	children,
	className,
	layerRef,
	placement = 'bottom-start',
	role,
	matchWidth = false
}: {
	open: boolean;
	children: ReactNode;
	className?: string;
	layerRef: RefObject<HTMLDivElement>;
	placement?: Placement;
	role?: string;
	matchWidth?: boolean;
}) {
	const origin =
		placement === 'right'
			? 'center left'
			: placement === 'left'
				? 'center right'
				: placement.startsWith('top')
					? placement.endsWith('end')
						? 'bottom right'
						: 'bottom left'
					: placement.startsWith('left')
						? 'top right'
						: placement.endsWith('end')
							? 'top right'
							: 'top left';
	const v = placement.startsWith('top') ? settle : rise;
	return (
		<AnimatePresence>
			{open && (
				<motion.div
					ref={layerRef}
					role={role}
					className={['layer', `layer-${placement}`, matchWidth && 'layer-match', className]
						.filter(Boolean)
						.join(' ')}
					style={{ transformOrigin: origin }}
					variants={v}
					initial="initial"
					animate="animate"
					exit="exit"
					transition={{ duration: 0.14, ease: [0.2, 0, 0, 1] }}
				>
					{children}
				</motion.div>
			)}
		</AnimatePresence>
	);
}
