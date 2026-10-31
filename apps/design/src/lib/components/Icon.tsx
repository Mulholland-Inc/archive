import type { LucideIcon, LucideProps } from 'lucide-react';
import { cx } from '../cx';

export type IconName = LucideIcon;
export type IconSize = 'sm' | 'md' | 'lg';

/* Every icon in the system goes through here so the stroke and the sizes
   are the tokens', never a component's own numbers.

   The caller's class is merged rather than spread over the top: a class
   arriving through the props used to replace `icon icon-md` outright, which
   dropped the icon out of its own sizing and left it at the library's raw
   24px. An icon with a class of its own is still an icon. */
export function Icon({
	icon: Glyph,
	size = 'md',
	className,
	...rest
}: { icon: LucideIcon; size?: IconSize } & Omit<LucideProps, 'size'>) {
	return (
		<Glyph
			className={cx('icon', `icon-${size}`, className)}
			strokeWidth="var(--icon-stroke)"
			absoluteStrokeWidth={false}
			aria-hidden="true"
			{...rest}
		/>
	);
}
