import { forwardRef, type InputHTMLAttributes, type TextareaHTMLAttributes } from 'react';
import type { LucideIcon } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Input.css';

export type InputSize = 'sm' | 'md' | 'lg';

export interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
	icon?: LucideIcon;
	size?: InputSize;
	invalid?: boolean;
	quiet?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
	{ icon, size = 'md', invalid, quiet, className, ...rest },
	ref
) {
	return (
		<span
			className={cx(
				'input',
				`input-${size}`,
				icon && 'input-with-icon',
				invalid && 'input-invalid',
				quiet && 'input-quiet',
				className
			)}
		>
			{icon && <Icon icon={icon} />}
			<input ref={ref} className="input-field" aria-invalid={invalid || undefined} {...rest} />
		</span>
	);
});

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
	invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
	{ invalid, className, rows = 3, ...rest },
	ref
) {
	return (
		<span className={cx('input input-area', invalid && 'input-invalid', className)}>
			<textarea
				ref={ref}
				className="input-field"
				rows={rows}
				aria-invalid={invalid || undefined}
				{...rest}
			/>
		</span>
	);
});
