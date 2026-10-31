import {
	cloneElement,
	useCallback,
	useMemo,
	useRef,
	useState,
	type ReactElement,
	type ReactNode
} from 'react';
import { Anchor, Layer, useDismiss, type Placement } from '../floating';
import './Popover.css';

/* Any content, anchored to a trigger. The trigger is the child; it gets a
   click and a ref. */
export function Popover({
	trigger,
	children,
	placement = 'bottom-start'
}: {
	trigger: ReactElement<{ onClick?: () => void; ref?: unknown }>;
	children: ReactNode;
	placement?: Placement;
}) {
	const [open, setOpen] = useState(false);
	const anchor = useRef<HTMLElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const close = useCallback(() => setOpen(false), []);
	const inside = useMemo(() => [anchor, layer], []);
	useDismiss(open, close, inside);
	return (
		<Anchor>
			{cloneElement(trigger, { ref: anchor, onClick: () => setOpen((o) => !o) })}
			<Layer open={open} layerRef={layer} placement={placement} className="popover" role="dialog">
				{children}
			</Layer>
		</Anchor>
	);
}
