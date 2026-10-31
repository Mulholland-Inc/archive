import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import {
	ArrowLeft,
	AtSign,
	Briefcase,
	Hash,
	Mail,
	MessageCircle,
	MessagesSquare,
	Phone,
	Smartphone
} from 'lucide-react';
import { IconButton } from './Button';
import { cx } from '../cx';
import { Avatar } from './Avatar';
import { Badge } from './Badge';
import { Icon } from './Icon';
import type { Hue } from '../hue';
import './Inbox.css';

export type ChannelId =
	'gmail' | 'outlook' | 'slack' | 'discord' | 'linkedin' | 'imessage' | 'whatsapp' | 'phone';

const channelWord: Record<ChannelId, string> = {
	gmail: 'Gmail',
	outlook: 'Outlook',
	slack: 'Slack',
	discord: 'Discord',
	linkedin: 'LinkedIn',
	imessage: 'iMessage',
	whatsapp: 'WhatsApp',
	phone: 'Phone'
};
const channelHue: Record<ChannelId, Hue> = {
	gmail: 'red',
	outlook: 'blue',
	slack: 'purple',
	discord: 'navy',
	linkedin: 'sky',
	imessage: 'green',
	whatsapp: 'teal',
	phone: 'gray'
};

const channelGlyph: Record<ChannelId, LucideIcon> = {
	gmail: Mail,
	outlook: AtSign,
	slack: Hash,
	discord: MessagesSquare,
	linkedin: Briefcase,
	imessage: MessageCircle,
	whatsapp: Smartphone,
	phone: Phone
};

/* Where a message came from: a line glyph in the channel's hue, its
   name on hover, or the word beside it on the channel's tint when there
   is room. Never a logo, never a box. */
export function Channel({ id, word = false }: { id: ChannelId; word?: boolean }) {
	if (word)
		return (
			<Badge hue={channelHue[id]} icon={channelGlyph[id]}>
				{channelWord[id]}
			</Badge>
		);
	return (
		<span
			className="channel"
			title={channelWord[id]}
			role="img"
			aria-label={channelWord[id]}
			style={{ color: `var(--hue-${channelHue[id]})` }}
		>
			<Icon icon={channelGlyph[id]} size="sm" />
		</span>
	);
}

export interface Talk {
	id: string;
	channel: ChannelId;
	/** Who is in it, besides us. */
	people: string[];
	/** The subject, or the first line where the channel has none. */
	title: string;
	/** The last thing said. */
	last: string;
	when: string;
	client?: string;
	unread?: boolean;
	/** The chat's own name where the channel has one: a Slack channel, a group. */
	place?: string;
}

/* Every conversation across every channel, one list, newest first.
   Three lines a row: who, with the channel and when at the end; what;
   and the last thing said, with the client after it. Unread is the
   weight of the words, nothing else. Squeezed narrow (a container query
   on the list's own width) the words go and a row is the person with
   the channel at their corner. */
export function Inbox({
	talks,
	selected,
	onSelect,
	className
}: {
	talks: Talk[];
	selected?: string;
	onSelect: (id: string) => void;
	className?: string;
}) {
	return (
		<div className={cx('inbox', className)}>
			{talks.map((t) => (
				<button
					key={t.id}
					type="button"
					className={cx(
						'talk chrome',
						t.unread && 'talk-unread',
						t.id === selected && 'talk-selected'
					)}
					onClick={() => onSelect(t.id)}
					aria-current={t.id === selected || undefined}
				>
					<span className="talk-who">
						<Avatar name={t.people[0]} size="lg" />
						<span className="talk-corner">
							<Channel id={t.channel} />
						</span>
					</span>
					<span className="talk-text">
						<span className="talk-head">
							<span className="talk-names">{t.people.join(', ')}</span>
							<span className="talk-side">
								<Channel id={t.channel} />
								<span className="talk-when">{t.when}</span>
							</span>
						</span>
						<span className="talk-title">{t.title}</span>
						<span className="talk-head">
							<span className="talk-last">{t.last}</span>
							{t.client && <span className="talk-client">{t.client}</span>}
						</span>
					</span>
				</button>
			))}
		</div>
	);
}

export interface Said {
	id: string;
	from: string;
	channel: ChannelId;
	when: string;
	text: string;
	/** Ours, not theirs. */
	mine?: boolean;
	attachments?: string[];
}

/* One conversation, whole: its title, who is in it, the client it is
   tagged to, then everything said in order with who, where and when.
   The reply goes back out the channel it came in on. */
export function Conversation({
	title,
	people,
	client,
	actions,
	back,
	said,
	footer,
	className
}: {
	title: string;
	people: string[];
	client?: ReactNode;
	actions?: ReactNode;
	/** The way back to the list, when the conversation is a page of its own. */ back?: () => void;
	said: Said[];
	footer?: ReactNode;
	className?: string;
}) {
	return (
		<div className={cx('conversation', className)}>
			<div className="conversation-head">
				<div className="conversation-title-row">
					{back && (
						<IconButton
							icon={ArrowLeft}
							label="Back to the list"
							size="sm"
							className="split-back"
							onClick={back}
						/>
					)}
					<h2 className="conversation-title">{title}</h2>
					{actions && <span className="conversation-actions">{actions}</span>}
				</div>
				<div className="conversation-meta chrome">
					<span className="conversation-people">{people.join(', ')}</span>
					{client}
				</div>
			</div>
			<div className="conversation-body">
				{said.map((m) => (
					<article key={m.id} className={cx('said', m.mine && 'said-mine')}>
						<Avatar name={m.from} />
						<div className="said-text">
							<div className="said-head chrome">
								<span className="said-from">{m.from}</span>
								<Channel id={m.channel} />
								<span className="said-when">{m.when}</span>
							</div>
							<p className="said-body">{m.text}</p>
							{m.attachments && m.attachments.length > 0 && (
								<div className="said-files">
									{m.attachments.map((a) => (
										<Badge key={a} hue="gray">
											{a}
										</Badge>
									))}
								</div>
							)}
						</div>
					</article>
				))}
			</div>
			{footer && <div className="conversation-foot">{footer}</div>}
		</div>
	);
}
