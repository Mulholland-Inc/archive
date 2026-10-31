import { useMemo, type ReactNode } from 'react';
import { AnimatePresence, LayoutGroup, motion } from 'motion/react';
import {
	MessageCircle,
	Mic,
	MicOff,
	MonitorUp,
	PhoneOff,
	Video,
	VideoOff,
	type LucideIcon
} from 'lucide-react';
import { cx } from '../cx';
import { Message, Thread } from './Message';
import { Avatar } from './Avatar';
import { Icon } from './Icon';
import './Call.css';

export interface Participant {
	id: string;
	name: string;
	muted?: boolean;
	camera?: boolean;
	speaking?: boolean;
	presenting?: boolean;
	me?: boolean;
}

const spring = { type: 'spring', stiffness: 380, damping: 36 } as const;

/* A call. Tiles fill a grid by how many are in it; a shared screen
   takes the stage and the people move to a strip beside it. Someone
   joining or leaving, or the stage changing shape, moves every tile to
   its new place on a spring. The controls run along the bottom, the
   chat opens as a pane on the right. Video, chat, screen: nothing else. */
export function Call({
	title,
	elapsed,
	people,
	mic,
	camera,
	sharing,
	chatOpen,
	onMic,
	onCamera,
	onShare,
	onChat,
	onLeave,
	chat,
	className
}: {
	title: string;
	elapsed?: string;
	people: Participant[];
	mic: boolean;
	camera: boolean;
	sharing: boolean;
	chatOpen?: boolean;
	onMic: () => void;
	onCamera: () => void;
	onShare: () => void;
	onChat?: () => void;
	onLeave: () => void;
	chat?: ReactNode;
	className?: string;
}) {
	const presenter = people.find((p) => p.presenting);
	const others = presenter ? people.filter((p) => p.id !== presenter.id) : people;
	// Rows of tiles: as many columns as the square root allows, the last
	// row taking whoever is left and stretching them, so the stage is
	// always full whatever the count.
	const rows = useMemo(() => {
		const n = others.length;
		const cols = Math.ceil(Math.sqrt(Math.max(1, n)));
		const count = Math.ceil(n / cols);
		const out: Participant[][] = [];
		for (let r = 0; r < count; r++) out.push(others.slice(r * cols, (r + 1) * cols));
		return out;
	}, [others]);
	return (
		<div className={cx('call', className)}>
			<div className="call-room">
				<div className="call-head chrome">
					<span className="call-title">{title}</span>
					{elapsed && <span className="call-elapsed">{elapsed}</span>}
				</div>
				<LayoutGroup>
					{presenter ? (
						<div className="call-stage-split">
							<motion.div
								className="call-screen"
								layoutId="call-screen"
								initial={{ opacity: 0 }}
								animate={{ opacity: 1 }}
								exit={{ opacity: 0 }}
								transition={spring}
							>
								<span className="call-screen-label chrome">
									{presenter.me ? 'You are presenting' : `${presenter.name} is presenting`}
								</span>
							</motion.div>
							<div className="call-strip">
								<AnimatePresence initial={false}>
									{others.map((p) => (
										<Tile key={p.id} p={p} />
									))}
								</AnimatePresence>
							</div>
						</div>
					) : (
						<div className="call-stage">
							<AnimatePresence initial={false}>
								{rows.map((row, i) => (
									<motion.div key={i} layout className="call-row" transition={spring}>
										{row.map((p) => (
											<Tile key={p.id} p={p} />
										))}
									</motion.div>
								))}
							</AnimatePresence>
						</div>
					)}
				</LayoutGroup>
				<div className="call-controls chrome">
					<Control
						icon={mic ? Mic : MicOff}
						label={mic ? 'Mute' : 'Unmute'}
						off={!mic}
						onClick={onMic}
					/>
					<Control
						icon={camera ? Video : VideoOff}
						label={camera ? 'Camera off' : 'Camera on'}
						off={!camera}
						onClick={onCamera}
					/>
					<Control
						icon={MonitorUp}
						label={sharing ? 'Stop presenting' : 'Present'}
						on={sharing}
						onClick={onShare}
					/>
					{onChat && <Control icon={MessageCircle} label="Chat" on={chatOpen} onClick={onChat} />}
					<button
						type="button"
						className="call-control call-leave"
						aria-label="Leave"
						onClick={onLeave}
					>
						<Icon icon={PhoneOff} />
					</button>
				</div>
			</div>
			{chatOpen && chat && <aside className="call-chat">{chat}</aside>}
		</div>
	);
}

/* A tile keeps its identity across the grid and the strip, so a change
   of stage moves it rather than remaking it. The mark and the name
   travel by position only, so they are never stretched on the way. */
function Tile({ p }: { p: Participant }) {
	return (
		<motion.div
			layout
			layoutId={`tile-${p.id}`}
			className={cx(
				'call-tile',
				p.speaking && 'call-tile-speaking',
				p.camera && 'call-tile-camera'
			)}
			initial={{ opacity: 0, scale: 0.92 }}
			animate={{ opacity: 1, scale: 1 }}
			exit={{ opacity: 0, scale: 0.92 }}
			transition={spring}
		>
			{!p.camera && (
				<motion.span layout="position" className="call-tile-mark-wrap">
					<Avatar name={p.name} size="lg" className="call-tile-mark" />
				</motion.span>
			)}
			<motion.span layout="position" className="call-tile-name chrome">
				{p.muted && <Icon icon={MicOff} size="sm" />}
				<span>{p.me ? 'You' : p.name}</span>
			</motion.span>
		</motion.div>
	);
}

function Control({
	icon,
	label,
	on,
	off,
	onClick
}: {
	icon: LucideIcon;
	label: string;
	on?: boolean;
	off?: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			className={cx('call-control', on && 'call-control-on', off && 'call-control-off')}
			aria-label={label}
			aria-pressed={on}
			title={label}
			onClick={onClick}
		>
			<Icon icon={icon} />
		</button>
	);
}

export interface Line {
	id: string;
	from: string;
	when: string;
	text: string;
	/** Said by us: sits in the tint on the right like any chat. */
	mine?: boolean;
}

/* What is said in the call: the turns, like any chat, with the composer
   at the foot. No box of its own; it is a column beside the room. */
export function CallChat({ lines, footer }: { lines: Line[]; footer?: ReactNode }) {
	return (
		<div className="callchat">
			<div className="callchat-lines">
				<Thread count={lines.length}>
					{lines.map((l) => (
						<Message
							key={l.id}
							role={l.mine ? 'user' : 'agent'}
							author={l.mine ? undefined : l.from}
							time={l.when}
						>
							{l.text}
						</Message>
					))}
				</Thread>
			</div>
			{footer && <div className="callchat-foot">{footer}</div>}
		</div>
	);
}
