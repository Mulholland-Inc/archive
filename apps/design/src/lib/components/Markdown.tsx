import { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { cx } from '../cx';
import './Markdown.css';

/* Text arriving in bursts, shown at an even pace: what is shown closes
   on what has arrived a fraction of the gap each frame, so a burst
   unrolls over a few frames and the tail lands a character at a time,
   never all at once. Off, the text is shown whole. */
export function useEased(text: string, on: boolean): string {
	const [shown, setShown] = useState(on ? 0 : text.length);
	const at = useRef(shown);
	useEffect(() => {
		if (!on) {
			at.current = text.length;
			setShown(text.length);
			return;
		}
		let frame = 0;
		const step = () => {
			const gap = text.length - at.current;
			if (gap <= 0) return;
			at.current = Math.min(text.length, at.current + Math.max(1, Math.ceil(gap / 10)));
			setShown(at.current);
			if (at.current < text.length) frame = requestAnimationFrame(step);
		};
		frame = requestAnimationFrame(step);
		return () => cancelAnimationFrame(frame);
	}, [text, on]);
	return on ? text.slice(0, Math.min(shown, text.length)) : text;
}

/* Words with their shape kept: headings at the base size and a heavier
   weight, lists, quotes, tables, links that open elsewhere, and code as
   a tint in the same face as everything else. What an agent writes.
   Streaming, the words come in at an even pace. */
export function Markdown({
	children,
	stream = false,
	className
}: {
	children: string;
	/** Still arriving: show it at an even pace. */ stream?: boolean;
	className?: string;
}) {
	const text = useEased(children, stream);
	return (
		<div className={cx('markdown', stream && 'markdown-stream', className)}>
			<ReactMarkdown
				remarkPlugins={[remarkGfm]}
				components={{
					a: ({ href, children: inner }) => (
						<a href={href} target="_blank" rel="noreferrer">
							{inner}
						</a>
					)
				}}
			>
				{text}
			</ReactMarkdown>
		</div>
	);
}
