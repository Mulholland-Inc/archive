import {
	createContext,
	useCallback,
	useContext,
	useMemo,
	useRef,
	useState,
	type ReactNode
} from 'react';
import { createPortal } from 'react-dom';
import { AnimatePresence, motion } from 'motion/react';
import { CircleAlert, CircleCheck, Info, X } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Toast.css';

export type ToastTone = 'neutral' | 'success' | 'danger';

export interface ToastOptions {
	tone?: ToastTone;
	action?: { label: string; onClick: () => void };
	duration?: number;
}

interface ToastRow extends ToastOptions {
	id: number;
	text: string;
}

const Ctx = createContext<(text: string, opts?: ToastOptions) => void>(() => {});

export function useToast() {
	return useContext(Ctx);
}

const glyph = { neutral: Info, success: CircleCheck, danger: CircleAlert };

/* Toasts stack at the bottom, newest last, and leave on their own. */
export function ToastProvider({ children }: { children: ReactNode }) {
	const [rows, setRows] = useState<ToastRow[]>([]);
	const seq = useRef(0);
	const dismiss = useCallback((id: number) => setRows((r) => r.filter((t) => t.id !== id)), []);
	const toast = useCallback(
		(text: string, opts: ToastOptions = {}) => {
			const id = ++seq.current;
			setRows((r) => [...r, { id, text, ...opts }]);
			window.setTimeout(() => dismiss(id), opts.duration ?? 4000);
		},
		[dismiss]
	);
	const value = useMemo(() => toast, [toast]);
	return (
		<Ctx.Provider value={value}>
			{children}
			{createPortal(
				<div className="toasts">
					<AnimatePresence initial={false}>
						{rows.map((t) => (
							<motion.div
								key={t.id}
								layout
								className={cx('toast', `toast-${t.tone ?? 'neutral'}`)}
								role="status"
								initial={{ opacity: 0, y: 12, scale: 0.98 }}
								animate={{ opacity: 1, y: 0, scale: 1 }}
								exit={{ opacity: 0, scale: 0.98 }}
								transition={{ type: 'spring', stiffness: 500, damping: 40 }}
							>
								<Icon icon={glyph[t.tone ?? 'neutral']} className="icon toast-glyph" />
								<span className="toast-text">{t.text}</span>
								{t.action && (
									<button
										type="button"
										className="toast-action chrome"
										onClick={() => {
											t.action?.onClick();
											dismiss(t.id);
										}}
									>
										{t.action.label}
									</button>
								)}
								<button
									type="button"
									className="toast-close chrome"
									aria-label="Dismiss"
									onClick={() => dismiss(t.id)}
								>
									<Icon icon={X} size="sm" />
								</button>
							</motion.div>
						))}
					</AnimatePresence>
				</div>,
				document.body
			)}
		</Ctx.Provider>
	);
}
