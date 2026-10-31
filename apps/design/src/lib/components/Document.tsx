import type { ReactNode } from 'react';
import { cx } from '../cx';
import { Cover } from './Cover';
import './Document.css';

/* A page as Notion lays one out: a banner across the top, the page's mark
   half over its edge, the title in the display size, and the words in one
   centred column of a comfortable measure.

   All of that is Cover, which a record uses too — a document is that plus
   the editor, at the reading size. */
export function Document({
	emoji,
	onEmoji,
	title,
	onTitle,
	cover,
	children,
	className
}: {
	emoji?: string;
	onEmoji?: (emoji: string) => void;
	title: string;
	onTitle?: (title: string) => void;
	/** A picture across the top; without one the banner is a tint. */ cover?: string;
	children: ReactNode;
	className?: string;
}) {
	return (
		<Cover
			cover={cover}
			emoji={emoji}
			onEmoji={onEmoji}
			title={title}
			onTitle={onTitle}
			className={cx('document', className)}
		>
			<div className="document-body">{children}</div>
		</Cover>
	);
}
