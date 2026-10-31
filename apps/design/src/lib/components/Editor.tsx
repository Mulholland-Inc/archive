import { useEffect } from 'react';
import {
	EditorContent,
	useEditor,
	useEditorState,
	type Editor as TiptapEditor
} from '@tiptap/react';
import { BubbleMenu } from '@tiptap/react/menus';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from '@tiptap/markdown';
import Placeholder from '@tiptap/extension-placeholder';
import TaskList from '@tiptap/extension-task-list';
import TaskItem from '@tiptap/extension-task-item';
import type { LucideIcon } from 'lucide-react';
import {
	Bold,
	Heading1,
	Heading2,
	Heading3,
	Italic,
	Link,
	List,
	ListChecks,
	ListOrdered,
	Minus,
	Pilcrow,
	Quote,
	Redo2,
	Strikethrough,
	Undo2
} from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Editor.css';

/* A document, written. Tiptap underneath, headless, so the words wear
   the system's type and nothing else. Markdown goes in and comes out;
   the marks are a row above and a small menu on whatever is selected.
   No code marks: the house has no monospace. */
export function Editor({
	value,
	onChange,
	placeholder = 'Write',
	readOnly = false,
	toolbar = true,
	className
}: {
	value: string;
	onChange?: (markdown: string) => void;
	placeholder?: string;
	readOnly?: boolean;
	toolbar?: boolean;
	className?: string;
}) {
	const editor = useEditor({
		extensions: [
			StarterKit.configure({ code: false, codeBlock: false, link: { openOnClick: false } }),
			Markdown,
			Placeholder.configure({ placeholder }),
			TaskList,
			TaskItem.configure({ nested: true })
		],
		content: value,
		contentType: 'markdown',
		editable: !readOnly,
		editorProps: { attributes: { class: 'editor-prose' } },
		onUpdate: ({ editor: e }) => onChange?.(e.getMarkdown())
	});
	useEffect(() => {
		editor?.setEditable(!readOnly);
	}, [editor, readOnly]);
	// A read only view follows its value; an editable one owns it.
	useEffect(() => {
		if (!editor || !readOnly) return;
		if (editor.getMarkdown() !== value)
			editor.commands.setContent(value, { contentType: 'markdown' });
	}, [editor, readOnly, value]);
	return (
		<div className={cx('editor', readOnly && 'editor-read', className)}>
			{toolbar && editor && !readOnly && <Marks editor={editor} />}
			{editor && !readOnly && (
				<BubbleMenu editor={editor} className="editor-bubble chrome" options={{ placement: 'top' }}>
					<Mark
						editor={editor}
						icon={Bold}
						label="Bold"
						active="bold"
						run={(e) => e.chain().focus().toggleBold().run()}
					/>
					<Mark
						editor={editor}
						icon={Italic}
						label="Italic"
						active="italic"
						run={(e) => e.chain().focus().toggleItalic().run()}
					/>
					<Mark
						editor={editor}
						icon={Strikethrough}
						label="Strike"
						active="strike"
						run={(e) => e.chain().focus().toggleStrike().run()}
					/>
					<Mark editor={editor} icon={Link} label="Link" active="link" run={(e) => link(e)} />
				</BubbleMenu>
			)}
			<EditorContent editor={editor} />
		</div>
	);
}

function link(e: TiptapEditor) {
	if (e.isActive('link')) return e.chain().focus().unsetLink().run();
	const href = window.prompt('Where to?');
	if (href) e.chain().focus().setLink({ href }).run();
	return true;
}

function Marks({ editor }: { editor: TiptapEditor }) {
	return (
		<div className="editor-marks chrome" role="toolbar">
			<Mark
				editor={editor}
				icon={Pilcrow}
				label="Text"
				active="paragraph"
				run={(e) => e.chain().focus().setParagraph().run()}
			/>
			<Mark
				editor={editor}
				icon={Heading1}
				label="Title"
				active={{ name: 'heading', attrs: { level: 1 } }}
				run={(e) => e.chain().focus().toggleHeading({ level: 1 }).run()}
			/>
			<Mark
				editor={editor}
				icon={Heading2}
				label="Heading"
				active={{ name: 'heading', attrs: { level: 2 } }}
				run={(e) => e.chain().focus().toggleHeading({ level: 2 }).run()}
			/>
			<Mark
				editor={editor}
				icon={Heading3}
				label="Subheading"
				active={{ name: 'heading', attrs: { level: 3 } }}
				run={(e) => e.chain().focus().toggleHeading({ level: 3 }).run()}
			/>
			<span className="editor-gap" />
			<Mark
				editor={editor}
				icon={Bold}
				label="Bold"
				active="bold"
				run={(e) => e.chain().focus().toggleBold().run()}
			/>
			<Mark
				editor={editor}
				icon={Italic}
				label="Italic"
				active="italic"
				run={(e) => e.chain().focus().toggleItalic().run()}
			/>
			<Mark
				editor={editor}
				icon={Strikethrough}
				label="Strike"
				active="strike"
				run={(e) => e.chain().focus().toggleStrike().run()}
			/>
			<Mark editor={editor} icon={Link} label="Link" active="link" run={(e) => link(e)} />
			<span className="editor-gap" />
			<Mark
				editor={editor}
				icon={List}
				label="List"
				active="bulletList"
				run={(e) => e.chain().focus().toggleBulletList().run()}
			/>
			<Mark
				editor={editor}
				icon={ListOrdered}
				label="Numbered"
				active="orderedList"
				run={(e) => e.chain().focus().toggleOrderedList().run()}
			/>
			<Mark
				editor={editor}
				icon={ListChecks}
				label="Tasks"
				active="taskList"
				run={(e) => e.chain().focus().toggleTaskList().run()}
			/>
			<Mark
				editor={editor}
				icon={Quote}
				label="Quote"
				active="blockquote"
				run={(e) => e.chain().focus().toggleBlockquote().run()}
			/>
			<Mark
				editor={editor}
				icon={Minus}
				label="Divider"
				run={(e) => e.chain().focus().setHorizontalRule().run()}
			/>
			<span className="editor-gap editor-gap-grow" />
			<Mark editor={editor} icon={Undo2} label="Undo" run={(e) => e.chain().focus().undo().run()} />
			<Mark editor={editor} icon={Redo2} label="Redo" run={(e) => e.chain().focus().redo().run()} />
		</div>
	);
}

function Mark({
	editor,
	icon,
	label,
	active,
	run
}: {
	editor: TiptapEditor;
	icon: LucideIcon;
	label: string;
	active?: string | { name: string; attrs: Record<string, unknown> };
	run: (e: TiptapEditor) => boolean | void;
}) {
	const on = useEditorState({
		editor,
		selector: ({ editor: e }) =>
			active
				? typeof active === 'string'
					? e.isActive(active)
					: e.isActive(active.name, active.attrs)
				: false
	});
	return (
		<button
			type="button"
			className={cx('editor-mark', on && 'editor-mark-on')}
			aria-label={label}
			aria-pressed={on}
			title={label}
			onMouseDown={(e) => e.preventDefault()}
			onClick={() => run(editor)}
		>
			<Icon icon={icon} size="sm" />
		</button>
	);
}
