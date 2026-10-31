import type { ReactNode } from 'react';
import { motion } from 'motion/react';
import { cx } from '../cx';
import { Brand } from './Brand';
import { Button } from './Button';
import { Spinner } from './Spinner';
import './Login.css';

export interface Workspace {
	id: string;
	name: string;
	/** The app it runs on: Dental, Accounting. */
	app?: string;
}

export type LoginStep = 'door' | 'workspaces';

const ease = [0.2, 0, 0, 1] as const;

/* The one door, and the page after it. First the two accounts a person
   can come in with; once they are in, the workspaces they belong to, one
   under the other. The step that is not showing folds to nothing, so the
   card eases to the height of the one that is; what is on it fades and
   slides, nothing is scaled. */
export function Login({
	mark,
	step,
	title = 'Sign in',
	text,
	onGoogle,
	onMicrosoft,
	onBrowser,
	browserLabel = 'Continue in your browser',
	busy = false,
	workspaces = [],
	onPick,
	account,
	onSignOut,
	footer,
	className
}: {
	mark?: ReactNode;
	step: LoginStep;
	title?: string;
	text?: string;
	onGoogle?: () => void;
	onMicrosoft?: () => void;
	/** One way in instead of two: the door is somewhere else and it asks which account itself. */ onBrowser?: () => void;
	browserLabel?: string;
	busy?: boolean;
	workspaces?: Workspace[];
	onPick?: (id: string) => void;
	account?: string;
	onSignOut?: () => void;
	footer?: ReactNode;
	className?: string;
}) {
	const door = step === 'door';
	return (
		<div className={cx('login', className)}>
			{mark && <div className="login-mark">{mark}</div>}
			<div className="login-steps">
				<div className={cx('login-fold', door && 'login-fold-open')}>
					<motion.div
						className="login-step"
						aria-hidden={!door}
						style={{ pointerEvents: door ? 'auto' : 'none' }}
						initial={false}
						animate={{ opacity: door ? 1 : 0 }}
						transition={{ duration: door ? 0.2 : 0.12, ease, delay: door ? 0.16 : 0 }}
					>
						<div className="login-head">
							<h1 className="login-title">{title}</h1>
							{text && <p className="login-text">{text}</p>}
						</div>
						<div className="login-slot">
							<motion.div
								className="login-doors"
								aria-hidden={busy}
								style={{ pointerEvents: busy ? 'none' : 'auto' }}
								initial={false}
								animate={{ opacity: busy ? 0 : 1 }}
								transition={{ duration: 0.18, ease }}
							>
								{/* A native client cannot show the two accounts, because it
							    does not do the asking: it hands the person to the door
							    in their own browser and that page asks. Offering a
							    Google button and a Microsoft button here would be two
							    names for one action. */}
								{onBrowser ? (
									<Button
										variant="outline"
										size="lg"
										onClick={onBrowser}
										className="login-door"
										tabIndex={door && !busy ? 0 : -1}
									>
										{browserLabel}
									</Button>
								) : (
									<>
										<Button
											variant="outline"
											size="lg"
											leading={<Brand name="google" />}
											onClick={onGoogle}
											className="login-door"
											tabIndex={door && !busy ? 0 : -1}
										>
											Continue with Google
										</Button>
										<Button
											variant="outline"
											size="lg"
											leading={<Brand name="microsoft" />}
											onClick={onMicrosoft}
											className="login-door"
											tabIndex={door && !busy ? 0 : -1}
										>
											Continue with Microsoft
										</Button>
									</>
								)}
							</motion.div>
							<motion.div
								className="login-busy"
								aria-hidden={!busy}
								initial={false}
								animate={{ opacity: busy ? 1 : 0 }}
								transition={{ duration: 0.18, ease }}
							>
								<Spinner size="md" />
								<span>Signing you in</span>
							</motion.div>
						</div>
						{footer && <div className="login-foot">{footer}</div>}
					</motion.div>
				</div>
				<div className={cx('login-fold', !door && 'login-fold-open')}>
					<motion.div
						className="login-step"
						aria-hidden={door}
						style={{ pointerEvents: door ? 'none' : 'auto' }}
						initial={false}
						animate={{ opacity: door ? 0 : 1 }}
						transition={{ duration: door ? 0.12 : 0.2, ease, delay: door ? 0 : 0.16 }}
					>
						<h1 className="login-title">Choose a workspace</h1>
						<div className="login-list">
							{workspaces.map((w, i) => (
								<motion.button
									key={w.id}
									type="button"
									className="login-row chrome"
									onClick={() => onPick?.(w.id)}
									tabIndex={door ? -1 : 0}
									initial={false}
									animate={{ opacity: door ? 0 : 1, y: door ? 4 : 0 }}
									transition={{ duration: 0.2, ease, delay: door ? 0 : 0.2 + i * 0.04 }}
								>
									<span className="login-row-name">{w.name}</span>
									{w.app && <span className="login-row-app">{w.app}</span>}
								</motion.button>
							))}
						</div>
						{(account || onSignOut) && (
							<div className="login-foot login-foot-row">
								{account && <span>Signed in as {account}</span>}
								{onSignOut && (
									<Button variant="ghost" size="sm" onClick={onSignOut} tabIndex={door ? -1 : 0}>
										Sign out
									</Button>
								)}
							</div>
						)}
					</motion.div>
				</div>
			</div>
		</div>
	);
}
