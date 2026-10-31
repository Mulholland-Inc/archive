import { useEffect, useState, type ReactNode, type Ref } from 'react';
import { ArrowLeft, ExternalLink, RotateCw } from 'lucide-react';
import { cx } from '../cx';
import { IconButton } from './Button';
import { Input } from './Input';
import './Browser.css';

/* A page beside the work: a way back, the address (Enter opens it), reload
   and a way out to the browser at the top, the page below. What
   the agent opens for the person to see, a change on localhost mostly.
   Natively the page is a webview laid over the body; in a browser the
   body holds whatever is given. */
export function Browser({
	url,
	onOpen,
	onBack,
	onReload,
	onOutside,
	bodyRef,
	children,
	className
}: {
	url: string;
	onOpen: (url: string) => void;
	onBack?: () => void;
	onReload?: () => void;
	onOutside?: () => void;
	bodyRef?: Ref<HTMLDivElement>;
	children?: ReactNode;
	className?: string;
}) {
	const [draft, setDraft] = useState(url);
	useEffect(() => setDraft(url), [url]);
	const go = () => {
		const v = draft.trim();
		if (!v) return;
		onOpen(/^[a-z]+:\/\//i.test(v) ? v : `https://${v}`);
	};
	return (
		<div className={cx('browser', className)}>
			<div className="browser-head chrome">
				{onBack && (
					<IconButton icon={ArrowLeft} label="Back" size="sm" onClick={onBack} disabled={!url} />
				)}
				<Input
					size="sm"
					value={draft}
					onChange={(e) => setDraft(e.target.value)}
					onKeyDown={(e) => e.key === 'Enter' && go()}
					placeholder="Address"
					className="browser-address"
				/>
				{onReload && (
					<IconButton icon={RotateCw} label="Reload" size="sm" onClick={onReload} disabled={!url} />
				)}
				{onOutside && (
					<IconButton
						icon={ExternalLink}
						label="Open in the browser"
						size="sm"
						onClick={onOutside}
						disabled={!url}
					/>
				)}
			</div>
			<div ref={bodyRef} className="browser-body">
				{children ??
					(!url && (
						<span className="browser-empty">
							Nothing open. Ask the agent to open a page, or type an address.
						</span>
					))}
			</div>
		</div>
	);
}
