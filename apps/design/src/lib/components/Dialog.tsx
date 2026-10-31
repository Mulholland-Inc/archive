import { useEffect, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { AnimatePresence, motion } from 'motion/react';
import { X } from 'lucide-react';
import { IconButton } from './Button';
import './Dialog.css';

/* A modal. Panel in front, Escape and the ground behind it close it.

   A dialog that asks something dims what it covers, because what it covers
   cannot be acted on until it is answered. A bare one — the command palette
   — does not: it is a place to type, the app behind it is still the thing
   being worked on, and a wash over the whole window to show a search field
   is a lot of ceremony for very little. The ground still closes it. */
export function Dialog({
	open,
	onClose,
	title,
	children,
	footer,
	bare = false
}: {
	open: boolean;
	onClose: () => void;
	title: string;
	children: ReactNode;
	footer?: ReactNode;
	/** No head: the body is the whole thing, the title is for the reader. */ bare?: boolean;
}) {
	const panel = useRef<HTMLDivElement | null>(null);
	useEffect(() => {
		if (!open) return;
		const key = (e: KeyboardEvent) => {
			if (e.key === 'Escape') onClose();
		};
		document.addEventListener('keydown', key);
		panel.current?.focus();
		return () => document.removeEventListener('keydown', key);
	}, [open, onClose]);
	return createPortal(
		<AnimatePresence>
			{open && (
				<motion.div
					className="dialog-root"
					initial={{ opacity: 0 }}
					animate={{ opacity: 1 }}
					exit={{ opacity: 0 }}
					transition={{ duration: 0.16 }}
				>
					<div
						className={bare ? 'dialog-scrim dialog-scrim-clear' : 'dialog-scrim'}
						onClick={onClose}
					/>
					<motion.div
						ref={panel}
						className="dialog"
						role="dialog"
						aria-modal="true"
						aria-label={title}
						tabIndex={-1}
						initial={{ opacity: 0, y: 8, scale: 0.98 }}
						animate={{ opacity: 1, y: 0, scale: 1 }}
						exit={{ opacity: 0, y: 8, scale: 0.98 }}
						transition={{ type: 'spring', stiffness: 500, damping: 40 }}
					>
						{!bare && (
							<div className="dialog-head">
								<h2 className="dialog-title">{title}</h2>
								<IconButton icon={X} label="Close" size="sm" onClick={onClose} />
							</div>
						)}
						<div className="dialog-body">{children}</div>
						{footer && <div className="dialog-foot">{footer}</div>}
					</motion.div>
				</motion.div>
			)}
		</AnimatePresence>,
		document.body
	);
}
