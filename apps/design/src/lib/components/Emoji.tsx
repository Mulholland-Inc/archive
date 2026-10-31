import { cx } from '../cx';
import './Emoji.css';

export type EmojiSize = 'sm' | 'md' | 'lg';

/* An emoji in an icon's box, so a row can carry either and line up the
   same. The glyph comes from the emoji font, never the text one, so it
   is the same picture on every machine that has it. */
export function Emoji({
	children,
	size = 'md',
	label,
	className
}: {
	children: string;
	size?: EmojiSize;
	label?: string;
	className?: string;
}) {
	return (
		<span
			className={cx('emoji', `emoji-${size}`, className)}
			role={label ? 'img' : undefined}
			aria-label={label}
			aria-hidden={label ? undefined : true}
		>
			{children}
		</span>
	);
}
