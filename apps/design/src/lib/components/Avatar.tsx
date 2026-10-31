import { cx } from '../cx';
import type { Hue } from '../hue';
import {
	Bird,
	Bone,
	Bug,
	Cat,
	Dog,
	Egg,
	Feather,
	Fish,
	Origami,
	Panda,
	PawPrint,
	Rabbit,
	Rat,
	Shell,
	Snail,
	Squirrel,
	Turtle,
	Worm,
	type LucideIcon
} from 'lucide-react';
import { Icon } from './Icon';
import './Avatar.css';

export type AvatarSize = 'sm' | 'md' | 'lg';

const hues: Hue[] = ['blue', 'green', 'amber', 'red', 'pink', 'purple', 'teal', 'navy'];
const faces: LucideIcon[] = [
	Rabbit,
	Bird,
	Cat,
	Dog,
	Fish,
	Squirrel,
	Snail,
	Turtle,
	Bug,
	Rat,
	Shell,
	Worm,
	PawPrint,
	Feather,
	Bone,
	Egg,
	Origami,
	Panda
];

/* A small stable hash of the name, so the same person always gets the
   same picture wherever they appear. */
function hash(text: string) {
	let h = 2166136261;
	for (let i = 0; i < text.length; i++) {
		h ^= text.charCodeAt(i);
		h = Math.imul(h, 16777619);
	}
	return h >>> 0;
}

/* A person: their photo, or an animal drawn from their name on a soft
   tint. The animal and the tint follow the name unless given, so the
   same person is the same rabbit everywhere. Never initials. */
export function Avatar({
	name,
	src,
	icon,
	hue,
	size = 'md',
	className
}: {
	name: string;
	src?: string;
	icon?: LucideIcon;
	hue?: Hue;
	size?: AvatarSize;
	className?: string;
}) {
	const h = hash(name);
	const tone = hue ?? hues[h % hues.length];
	const face = icon ?? faces[(h >>> 8) % faces.length];
	return (
		<span
			className={cx('avatar chrome', `avatar-${size}`, !src && 'avatar-mark', className)}
			style={
				!src
					? ({
							'--avatar-soft': `var(--hue-${tone}-soft)`,
							'--avatar-ink': `var(--hue-${tone}-text)`
						} as React.CSSProperties)
					: undefined
			}
			title={name}
			role="img"
			aria-label={name}
		>
			{src ? <img className="avatar-image" src={src} alt="" /> : <Icon icon={face} />}
		</span>
	);
}
