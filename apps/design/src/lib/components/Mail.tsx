import { useState, type ReactNode } from 'react';
import { AlarmClock, Paperclip, Send, X } from 'lucide-react';
import { cx } from '../cx';
import { Avatar } from './Avatar';
import { Button, IconButton } from './Button';
import { Icon } from './Icon';
import { Select } from './Select';
import './Mail.css';

export interface Address {
	id: string;
	name: string;
	email: string;
}

export interface Draft {
	from: string;
	to: Address[];
	cc: Address[];
	bcc: Address[];
	subject: string;
	body: string;
}

/* A mail being written: who it is from, who it is to, what it is
   about, then the words. The lines read as a form only where you are;
   everywhere else they read as the letter. Cc and Bcc stay out of the
   way until asked for. */
export function Mail({
	draft,
	onChange,
	accounts,
	onSend,
	onAttach,
	onSchedule,
	sending = false,
	attachments = [],
	onRemoveAttachment,
	header,
	className
}: {
	draft: Draft;
	onChange: (draft: Draft) => void;
	accounts: Array<{ value: string; label: string }>;
	onSend: () => void;
	onAttach?: () => void;
	onSchedule?: () => void;
	sending?: boolean;
	attachments?: Array<{ id: string; name: string }>;
	onRemoveAttachment?: (id: string) => void;
	header?: ReactNode;
	className?: string;
}) {
	const [more, setMore] = useState<{ cc: boolean; bcc: boolean }>({
		cc: draft.cc.length > 0,
		bcc: draft.bcc.length > 0
	});
	const set = <K extends keyof Draft>(key: K, value: Draft[K]) =>
		onChange({ ...draft, [key]: value });
	const ready =
		draft.to.length > 0 &&
		draft.subject.trim().length > 0 &&
		draft.body.trim().length > 0 &&
		!sending;
	return (
		<div className={cx('mail', className)}>
			{header && <div className="mail-head">{header}</div>}
			<div className="mail-lines">
				<div className="mail-line">
					<span className="mail-label chrome">From</span>
					<span className="mail-control">
						<Select
							quiet
							size="sm"
							value={draft.from}
							onChange={(v) => set('from', v)}
							options={accounts}
						/>
					</span>
				</div>
				<People
					label="To"
					people={draft.to}
					onRemove={(id) =>
						set(
							'to',
							draft.to.filter((p) => p.id !== id)
						)
					}
					trailing={
						(!more.cc || !more.bcc) && (
							<span className="mail-more chrome">
								{!more.cc && (
									<button
										type="button"
										className="mail-more-link"
										onClick={() => setMore((m) => ({ ...m, cc: true }))}
									>
										Cc
									</button>
								)}
								{!more.bcc && (
									<button
										type="button"
										className="mail-more-link"
										onClick={() => setMore((m) => ({ ...m, bcc: true }))}
									>
										Bcc
									</button>
								)}
							</span>
						)
					}
				/>
				{more.cc && (
					<People
						label="Cc"
						people={draft.cc}
						onRemove={(id) =>
							set(
								'cc',
								draft.cc.filter((p) => p.id !== id)
							)
						}
					/>
				)}
				{more.bcc && (
					<People
						label="Bcc"
						people={draft.bcc}
						onRemove={(id) =>
							set(
								'bcc',
								draft.bcc.filter((p) => p.id !== id)
							)
						}
					/>
				)}
				<div className="mail-line">
					<span className="mail-label chrome">Subject</span>
					<input
						className="mail-subject"
						value={draft.subject}
						placeholder="What it is about"
						onChange={(e) => set('subject', e.target.value)}
					/>
				</div>
			</div>
			<div className="mail-body">
				<span className="mail-grow" data-value={draft.body}>
					<textarea
						className="mail-text"
						value={draft.body}
						placeholder="Write"
						rows={6}
						onChange={(e) => set('body', e.target.value)}
					/>
				</span>
			</div>
			{attachments.length > 0 && (
				<div className="mail-files">
					{attachments.map((a) => (
						<span key={a.id} className="mail-file chrome">
							<Icon icon={Paperclip} size="sm" />
							<span>{a.name}</span>
							{onRemoveAttachment && (
								<button
									type="button"
									className="mail-file-remove"
									aria-label={`Remove ${a.name}`}
									onClick={() => onRemoveAttachment(a.id)}
								>
									<Icon icon={X} size="sm" />
								</button>
							)}
						</span>
					))}
				</div>
			)}
			<div className="mail-foot">
				<span className="mail-tools">
					{onAttach && <IconButton icon={Paperclip} label="Attach" size="sm" onClick={onAttach} />}
					{onSchedule && (
						<IconButton icon={AlarmClock} label="Send later" size="sm" onClick={onSchedule} />
					)}
				</span>
				<Button variant="primary" icon={Send} disabled={!ready} loading={sending} onClick={onSend}>
					Send
				</Button>
			</div>
		</div>
	);
}

function People({
	label,
	people,
	onRemove,
	trailing
}: {
	label: string;
	people: Address[];
	onRemove: (id: string) => void;
	trailing?: ReactNode;
}) {
	return (
		<div className="mail-line">
			<span className="mail-label chrome">{label}</span>
			<span className="mail-people">
				{people.map((p) => (
					<span key={p.id} className="mail-person chrome" title={p.email}>
						<Avatar name={p.name} size="sm" />
						<span>{p.name}</span>
						<button
							type="button"
							className="mail-person-remove"
							aria-label={`Remove ${p.name}`}
							onClick={() => onRemove(p.id)}
						>
							<Icon icon={X} size="sm" />
						</button>
					</span>
				))}
				{people.length === 0 && <span className="mail-empty">Add someone</span>}
			</span>
			{trailing}
		</div>
	);
}
