import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { cx } from '../cx';
import './Split.css';

/* Two panes side by side with a seam between them that can be dragged.
   The first pane has the width; the second takes the rest. Nothing is
   measured: the width moves by how far the pointer went. Past its
   limits the pane gives a little, like a rubber band, and when let go
   it springs back; dragged under a threshold it springs to a compact
   width, so the pane inside can fold to its compact form (a container
   query on its own width). The width is kept under the id between pages and launches.
   With `stack`, under 480 of its own width the seam goes and one pane
   fills it, whichever `show` names, so the two become pages you go
   between, the way a phone's messages do. */
export interface Snap {
	/** Under this width the pane snaps to `to`. */
	below: number;
	/** The compact width. */
	to: number;
}

const readWidth = (id: string) => {
	try {
		const saved = Number(localStorage.getItem(`split:${id}`));
		return Number.isFinite(saved) && saved > 0 ? saved : null;
	} catch {
		return null;
	}
};

const keepWidth = (id: string, width: number) => {
	try {
		localStorage.setItem(`split:${id}`, String(width));
	} catch {
		/* a private window */
	}
};

export function Split({
	id,
	width: initial,
	onWidth,
	min,
	max,
	snap,
	stack = false,
	show = 'first',
	side = 'start',
	children,
	className
}: {
	id: string;
	width: number;
	/** Given, the width is the caller's: this reports a drag and nothing is kept. */ onWidth?: (
		width: number
	) => void;
	min: number;
	max: number;
	snap?: Snap;
	stack?: boolean;
	show?: 'first' | 'rest';
	/** Which pane the width belongs to; the other takes the rest. */ side?: 'start' | 'end';
	children: [ReactNode, ReactNode];
	className?: string;
}) {
	const clamp = (w: number) => {
		if (snap && w < snap.below) return snap.to;
		return Math.min(max, Math.max(min, w));
	};
	// While dragging the pane follows the hand, giving a quarter of the
	// way past its limits; the snap and the limits take hold on release.
	const give = (w: number) => {
		const floor = snap ? snap.to : min;
		if (w > max) return max + (w - max) / 4;
		if (w < floor) return floor + (w - floor) / 4;
		return w;
	};
	const [kept, setKept] = useState(() => readWidth(id) ?? initial);
	const width = onWidth ? initial : kept;
	const setWidth = onWidth ?? setKept;
	useEffect(() => {
		if (!onWidth) keepWidth(id, kept);
	}, [id, kept, onWidth]);
	const start = useRef({ x: 0, width });
	const [dragging, setDragging] = useState(false);
	return (
		<div
			className={cx(
				'split',
				side === 'end' && 'split-end',
				dragging && 'split-dragging',
				stack && 'split-stack',
				stack && `split-show-${show}`,
				className
			)}
			style={{ '--split-width': `${width}px` } as CSSProperties}
		>
			<div className="split-first">{children[0]}</div>
			<div
				className="split-seam"
				role="separator"
				aria-orientation="vertical"
				aria-valuenow={width}
				aria-valuemin={snap ? snap.to : min}
				aria-valuemax={max}
				onPointerDown={(e) => {
					start.current = { x: e.clientX, width };
					setDragging(true);
					e.currentTarget.setPointerCapture(e.pointerId);
				}}
				onPointerMove={(e) => {
					if (!dragging) return;
					setWidth(
						give(start.current.width + (side === 'end' ? -1 : 1) * (e.clientX - start.current.x))
					);
				}}
				onPointerUp={(e) => {
					setDragging(false);
					setWidth(
						clamp(start.current.width + (side === 'end' ? -1 : 1) * (e.clientX - start.current.x))
					);
					e.currentTarget.releasePointerCapture(e.pointerId);
				}}
				onDoubleClick={() => setWidth(initial)}
			/>
			<div className="split-rest">{children[1]}</div>
		</div>
	);
}
