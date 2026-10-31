import { useState, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { Check, ChevronDown, ChevronRight } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import { Spinner } from './Spinner';
import './Jobs.css';

export type JobState = 'running' | 'done' | 'queued';

export interface Job {
	id: string;
	title: string;
	verb: string;
	detail?: string;
	state: JobState;
}

/* What the agent is doing right now. The head is one line, a press on it
   folds the list under it open or closed. Each job carries its verb in
   colour and what it is on; the queued ones fade below. */
export function Jobs({
	title,
	caption,
	summary,
	jobs,
	onOpen,
	leading,
	open: openProp,
	defaultOpen = true,
	onToggle,
	className
}: {
	title: string;
	caption?: string;
	summary?: string;
	jobs: Job[];
	onOpen?: (id: string) => void;
	leading?: ReactNode;
	open?: boolean;
	defaultOpen?: boolean;
	onToggle?: (open: boolean) => void;
	className?: string;
}) {
	const [own, setOwn] = useState(defaultOpen);
	const open = openProp ?? own;
	const toggle = () => {
		setOwn(!open);
		onToggle?.(!open);
	};
	return (
		<div className={cx('jobs', className)}>
			<button type="button" className="jobs-head chrome" aria-expanded={open} onClick={toggle}>
				{leading}
				<span className="jobs-text">
					<span className="jobs-title">{title}</span>
					{caption && <span className="jobs-caption">{caption}</span>}
				</span>
				{summary && <span className="jobs-summary">{summary}</span>}
				<motion.span
					className="jobs-fold"
					animate={{ rotate: open ? 180 : 0 }}
					transition={{ duration: 0.2, ease: [0.2, 0, 0, 1] }}
				>
					<Icon icon={ChevronDown} />
				</motion.span>
			</button>
			<AnimatePresence initial={false}>
				{open && (
					<motion.div
						className="jobs-fold-body"
						initial={{ height: 0, opacity: 0 }}
						animate={{ height: 'auto', opacity: 1 }}
						exit={{ height: 0, opacity: 0 }}
						transition={{ duration: 0.24, ease: [0.2, 0, 0, 1] }}
					>
						<div className="jobs-list">
							{jobs.map((j) => (
								<motion.button
									key={j.id}
									layout
									type="button"
									className={cx('job chrome', `job-${j.state}`)}
									onClick={() => onOpen?.(j.id)}
									disabled={j.state === 'queued'}
									transition={{ type: 'spring', stiffness: 500, damping: 40 }}
								>
									<span className="job-glyph">
										{j.state === 'done' ? <Icon icon={Check} /> : <Spinner size="md" />}
									</span>
									<span className="job-text">
										<span className="job-title">{j.title}</span>
										<span className="job-line">
											<span className="job-verb">{j.verb}</span>
											{j.detail && <span className="job-detail">{j.detail}</span>}
										</span>
									</span>
									<Icon icon={ChevronRight} className="icon job-chevron" />
								</motion.button>
							))}
						</div>
					</motion.div>
				)}
			</AnimatePresence>
		</div>
	);
}
