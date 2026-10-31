import { useRef, type ReactNode } from 'react';
import type { Hue } from '../hue';
import { cx } from '../cx';
import { Emoji } from './Emoji';
import { EmojiPicker } from './EmojiPicker';
import './Cover.css';

/* The top of a page someone made: a banner edge to edge, the page's emoji
   half over its lower edge, and the name below it.

   A document and a client record are the same thing seen twice — a banner,
   a mark, a name, then whatever the page is — so they are one component
   rather than two that drift. What differs is only the column under it: a
   document reads at a measure, a record lays out properties. That is the
   parent's business, and it comes in as `children`.

   Without a picture the banner is a tint, which is what a page wears until
   someone gives it one. */
export function Cover({
	cover,
	hue,
	emoji,
	onEmoji,
	title,
	onTitle,
	placeholder = 'Untitled',
	tools,
	children,
	className
}: {
	/** A picture across the top; without one the banner is a tint. */
	cover?: string;
	/** The tint the banner falls away from, when there is no picture. */
	hue?: Hue;
	emoji?: string;
	/** Given, the mark can be changed in place. */
	onEmoji?: (emoji: string) => void;
	title: string;
	/** Given, the name can be changed in place. */
	onTitle?: (title: string) => void;
	placeholder?: string;
	/** The corner of the banner: a way back, a way out. */
	tools?: ReactNode;
	children: ReactNode;
	className?: string;
}) {
	const heading = useRef<HTMLHeadingElement>(null);
	const face = emoji ? <Emoji className="cover-emoji">{emoji}</Emoji> : null;
	return (
		<article className={cx('cover', className)}>
			<div
				className="cover-band"
				style={{
					...(cover ? { backgroundImage: `url(${cover})` } : undefined),
					...(hue
						? ({
								'--cover-tint': `linear-gradient(135deg, var(--hue-${hue}-soft), var(--muted))`
							} as React.CSSProperties)
						: undefined)
				}}
			>
				{tools && <div className="cover-tools chrome">{tools}</div>}
			</div>
			<div className="cover-column">
				<div className="cover-head">
					{face &&
						(onEmoji ? (
							<EmojiPicker
								value={emoji}
								onChange={onEmoji}
								trigger={
									<button type="button" className="cover-face chrome" aria-label="Change the emoji">
										{face}
									</button>
								}
							/>
						) : (
							<span className="cover-face">{face}</span>
						))}
					<h1
						ref={heading}
						className={cx('cover-title', !title && 'cover-title-empty')}
						contentEditable={onTitle ? 'plaintext-only' : undefined}
						suppressContentEditableWarning
						data-placeholder={placeholder}
						onBlur={() => onTitle?.(heading.current?.textContent?.trim() ?? '')}
						onKeyDown={(e) => {
							if (e.key === 'Enter') {
								e.preventDefault();
								heading.current?.blur();
							}
						}}
					>
						{title}
					</h1>
				</div>
				{children}
			</div>
		</article>
	);
}
