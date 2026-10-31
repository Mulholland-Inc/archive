import { useCallback, useMemo, useRef, useState, type ReactElement } from 'react';
import { cloneElement } from 'react';
import { Anchor, Layer, useDismiss, useFlip, type Placement } from '../floating';
import { cx } from '../cx';
import { Emoji } from './Emoji';
import './EmojiPicker.css';

const faces = [
	'📄',
	'📝',
	'📋',
	'📊',
	'📈',
	'🗂️',
	'🗃️',
	'📁',
	'🏭',
	'🏪',
	'🏥',
	'🏦',
	'🏫',
	'🏗️',
	'🧱',
	'🛤️',
	'🦷',
	'😁',
	'💊',
	'🩺',
	'🧾',
	'💸',
	'💰',
	'🧮',
	'🎯',
	'🚀',
	'🚩',
	'✅',
	'⚡',
	'💡',
	'🔬',
	'🧠',
	'📞',
	'📹',
	'💬',
	'✉️',
	'📅',
	'⏰',
	'🗺️',
	'🧭',
	'👗',
	'🛍️',
	'📦',
	'🚚',
	'🔨',
	'🎨',
	'🧩',
	'✨'
];

/* A grid of emoji under a trigger, to pick one for a page or a thing.
   Notion's picker, cut to the emoji this house uses. The one in force
   is on a tint. */
export function EmojiPicker({
	value,
	onChange,
	trigger,
	placement = 'bottom-start'
}: {
	value?: string;
	onChange: (emoji: string) => void;
	trigger: ReactElement<{ onClick?: () => void; ref?: unknown }>;
	placement?: Placement;
}) {
	const [open, setOpen] = useState(false);
	const anchor = useRef<HTMLElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const close = useCallback(() => setOpen(false), []);
	const inside = useMemo(() => [anchor, layer], []);
	useDismiss(open, close, inside);
	const side = useFlip(open, anchor, placement);
	return (
		<Anchor>
			{cloneElement(trigger, { ref: anchor, onClick: () => setOpen((o) => !o) })}
			<Layer
				open={open}
				layerRef={layer}
				placement={side}
				className="emoji-picker chrome"
				role="dialog"
			>
				<div className="emoji-grid">
					{faces.map((f) => (
						<button
							key={f}
							type="button"
							className={cx('emoji-pick', f === value && 'emoji-pick-on')}
							aria-label={f}
							aria-pressed={f === value}
							onClick={() => {
								onChange(f);
								setOpen(false);
							}}
						>
							<Emoji>{f}</Emoji>
						</button>
					))}
				</div>
			</Layer>
		</Anchor>
	);
}
