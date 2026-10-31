import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import { Spinner } from './Spinner';
import './Button.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'outline' | 'destructive';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	size?: ButtonSize;
	icon?: LucideIcon;
	/** Anything that is not a Lucide icon in the icon's place: a brand mark, a face. */
	leading?: ReactNode;
	trailing?: LucideIcon;
	loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
	{
		variant = 'secondary',
		size = 'md',
		icon,
		leading,
		trailing,
		loading = false,
		className,
		children,
		disabled,
		...rest
	},
	ref
) {
	return (
		<button
			ref={ref}
			type="button"
			className={cx(
				'button chrome',
				`button-${variant}`,
				`button-${size}`,
				loading && 'button-loading',
				className
			)}
			disabled={disabled || loading}
			aria-busy={loading || undefined}
			{...rest}
		>
			{loading ? <Spinner size="sm" /> : icon ? <Icon icon={icon} /> : leading}
			{children && <span className="button-label">{children}</span>}
			{trailing && <Icon icon={trailing} />}
		</button>
	);
});

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	icon?: LucideIcon;
	label: string;
	variant?: ButtonVariant;
	size?: ButtonSize;
}

/* A button that is only its icon. The label is for the screen reader and
   the tooltip, never painted. */
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
	{ icon, label, variant = 'ghost', size = 'md', className, ...rest },
	ref
) {
	return (
		<button
			ref={ref}
			type="button"
			className={cx('button button-icon chrome', `button-${variant}`, `button-${size}`, className)}
			aria-label={label}
			{...rest}
		>
			{icon && <Icon icon={icon} />}
		</button>
	);
});
