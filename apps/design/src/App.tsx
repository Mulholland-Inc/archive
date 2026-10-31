import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
	Image as ImageGlyph,
	LogOut,
	Antenna,
	AppWindow,
	Archive,
	ArrowRight,
	Ban,
	Banknote,
	Bell,
	BookMarked,
	Bot,
	BrickWall,
	Briefcase,
	Calendar,
	CalendarDays,
	ChartColumn,
	ChevronDown,
	ChevronRight,
	ChevronsUpDown,
	CircleCheck,
	CircleDot,
	CircleQuestionMark,
	Clipboard,
	Clock,
	Compass,
	Container,
	Copy,
	Cpu,
	Ellipsis,
	Eraser,
	Eye,
	File as FileGlyph,
	FileText,
	Film,
	Filter,
	Flag,
	Flower,
	Flower2,
	Folder,
	Folders,
	Forward,
	Fuel,
	Gauge,
	GitBranch,
	GitFork,
	Globe,
	Hammer,
	Hospital,
	Hourglass,
	Inbox as InboxGlyph,
	InboxIcon,
	Key,
	Keyboard,
	Layers,
	LayoutGrid,
	Lightbulb,
	Link,
	ListFilter,
	LoaderCircle,
	Mail,
	MailOpen,
	Map as MapGlyph,
	MapPin,
	MessageCircle,
	Minus,
	Moon,
	MousePointer2,
	MoveHorizontal,
	MoveRight,
	Palette,
	Paperclip,
	PenLine,
	Pencil,
	Phone,
	Plus,
	Puzzle,
	Reply,
	Rocket,
	Route,
	Ruler,
	Search,
	Settings,
	SlidersHorizontal,
	Smile,
	Sparkles,
	Spool,
	Square,
	SquareCheck,
	Star,
	Stethoscope,
	Sticker,
	StickyNote,
	Store,
	Sun,
	Table2,
	Tag,
	Terminal,
	Toolbox,
	Trash2,
	TrendingUp,
	Type,
	Upload,
	Users,
	Video,
	Zap
} from 'lucide-react';
import {
	Action,
	Actions,
	Ask,
	Avatar,
	Badge,
	Board,
	Pipeline,
	Browser,
	Brand,
	Breadcrumb,
	Button,
	Calendar as MonthCalendar,
	Call,
	CallChat,
	Callout,
	Cover,
	CardBody,
	Channel,
	Conversation,
	AgentColumn,
	AgentTurn,
	agentKinds,
	Checkbox,
	Commands,
	Composer,
	Dialog,
	Document,
	Editor,
	Elicit,
	Empty,
	Field,
	Header,
	Icon,
	IconButton,
	Inbox,
	Input,
	Jobs,
	List,
	ListRow,
	Login,
	Mail as MailComposer,
	Markdown,
	Menu,
	Message,
	Meter,
	Panel,
	Plan,
	Popover,
	Property,
	Progress,
	RadioGroup,
	Record,
	RecordBody,
	RecordDivider,
	RecordLink,
	RecordLinks,
	RecordProperties,
	RecordProperty,
	Segmented,
	Select,
	Sheet,
	Sidebar,
	SidebarFoot,
	SidebarHead,
	Workspace,
	Emoji,
	EmojiPicker,
	Shell,
	Spinner,
	Split,
	Swatch,
	Swatches,
	Step,
	Switch,
	Table,
	Tabs,
	Targets,
	TargetsBar,
	Score,
	Tile,
	Tool,
	ToolDivider,
	ToolField,
	ToolValue,
	Toolbar,
	Textarea,
	Thinking,
	Thread,
	Timeline,
	ToastProvider,
	Tooltip,
	Usage,
	resetsIn,
	useToast,
	type LoginStep,
	type BoardCard,
	type Engagement,
	type Stage,
	type CommandItem,
	type Attached,
	type SessionView,
	type Draft,
	type Target,
	type Participant,
	type ChannelId,
	type Said,
	type Talk,
	type Entry,
	type CalendarEvent,
	type CalendarView,
	type Column,
	type Hue,
	type Job,
	type MenuEntry,
	type Snap,
	type SheetChange,
	type SheetColumn,
	type SheetRow,
	type SidebarItem,
	type SidebarSection
} from './lib';

const sections: SidebarSection[] = [
	{
		id: 'foundations',
		label: 'Foundations',
		items: [
			{ id: 'color', label: 'Color', icon: Palette },
			{ id: 'typography', label: 'Typography', icon: Type },
			{ id: 'spacing', label: 'Spacing', icon: Ruler },
			{ id: 'motion', label: 'Motion', icon: Film },
			{ id: 'icons', label: 'Icons', icon: Sparkles }
		]
	},
	{
		id: 'actions',
		label: 'Actions',
		items: [
			{ id: 'button', label: 'Button', icon: CircleDot },
			{ id: 'icon-button', label: 'Icon button', icon: Square },
			{ id: 'menu', label: 'Menu', icon: Clipboard }
		]
	},
	{
		id: 'forms',
		label: 'Forms',
		items: [
			{ id: 'input', label: 'Input', icon: Pencil },
			{ id: 'select', label: 'Select', icon: ChevronDown },
			{ id: 'checkbox', label: 'Checkbox', icon: SquareCheck },
			{ id: 'radio', label: 'Radio', icon: CircleDot },
			{ id: 'switch', label: 'Switch', icon: SlidersHorizontal }
		]
	},
	{
		id: 'navigation',
		label: 'Navigation',
		items: [
			{ id: 'tabs', label: 'Tabs', icon: Folders },
			{ id: 'segmented', label: 'Segmented', icon: Puzzle },
			{ id: 'breadcrumb', label: 'Breadcrumb', icon: Bell },
			{ id: 'sidebar', label: 'Sidebar', icon: BookMarked }
		]
	},
	{
		id: 'feedback',
		label: 'Feedback',
		items: [
			{ id: 'spinner', label: 'Spinner', icon: LoaderCircle },
			{ id: 'progress', label: 'Progress', icon: TrendingUp },
			{ id: 'badge', label: 'Badge', icon: Tag },
			{ id: 'callout', label: 'Callout', icon: Lightbulb },
			{ id: 'tooltip', label: 'Tooltip', icon: MessageCircle },
			{ id: 'toast', label: 'Toast', icon: Bell },
			{ id: 'empty', label: 'Empty', icon: Container }
		]
	},
	{
		id: 'chat',
		label: 'Chat',
		items: [
			{ id: 'composer', label: 'Composer', icon: PenLine },
			{ id: 'message', label: 'Message', icon: MessageCircle },
			{ id: 'thread', label: 'Thread', icon: Spool },
			{ id: 'markdown', label: 'Markdown', icon: StickyNote }
		]
	},
	{
		id: 'shell',
		label: 'Shell',
		items: [
			{ id: 'header', label: 'Header', icon: Compass },
			{ id: 'commands', label: 'Commands', icon: Keyboard },
			{ id: 'split', label: 'Split', icon: MoveHorizontal },
			{ id: 'browser', label: 'Browser', icon: Globe },
			{ id: 'usage', label: 'Usage', icon: Fuel }
		]
	},
	{ id: 'access', label: 'Access', items: [{ id: 'login', label: 'Login', icon: Key }] },
	{
		id: 'inbox',
		label: 'Inbox',
		items: [
			{ id: 'talks', label: 'Inbox', icon: InboxGlyph },
			{ id: 'conversation', label: 'Conversation', icon: MessageCircle },
			{ id: 'mail', label: 'Mail', icon: Mail },
			{ id: 'channel', label: 'Channel', icon: Antenna }
		]
	},
	{
		id: 'coding',
		label: 'Coding',
		items: [
			{ id: 'session', label: 'Session', icon: Bot },
			{ id: 'actions', label: 'Actions', icon: Zap },
			{ id: 'ask', label: 'Ask', icon: CircleQuestionMark },
			{ id: 'plan', label: 'Plan', icon: MapGlyph },
			{ id: 'elicit', label: 'Elicit', icon: StickyNote }
		]
	},
	{
		id: 'overlay',
		label: 'Overlay',
		items: [
			{ id: 'dialog', label: 'Dialog', icon: AppWindow },
			{ id: 'popover', label: 'Popover', icon: Sticker }
		]
	},
	{
		id: 'dessin',
		label: 'Dessin',
		items: [
			{ id: 'panel', label: 'Panel', icon: BrickWall },
			{ id: 'toolbar', label: 'Toolbar', icon: Toolbox },
			{ id: 'tile', label: 'Tile', icon: BrickWall },
			{ id: 'jobs', label: 'Jobs', icon: Settings },
			{ id: 'invite', label: 'Invite', icon: FileGlyph }
		]
	},
	{
		id: 'crm',
		label: 'CRM',
		items: [
			{ id: 'record', label: 'Record', icon: Archive },
			{ id: 'board', label: 'Board', icon: Folders },
			{ id: 'pipeline', label: 'Pipeline', icon: Route },
			{ id: 'outreach', label: 'Outreach', icon: FileGlyph },
			{ id: 'calendar', label: 'Calendar', icon: Calendar },
			{ id: 'call', label: 'Call', icon: Video },
			{ id: 'timeline', label: 'Timeline', icon: Clock },
			{ id: 'meter', label: 'Meter', icon: Terminal }
		]
	},
	{
		id: 'data',
		label: 'Data',
		items: [
			{ id: 'avatar', label: 'Avatar', icon: Smile },
			{ id: 'list', label: 'List', icon: FileText },
			{ id: 'table', label: 'Table', icon: ChartColumn },
			{ id: 'sheet', label: 'Sheet', icon: ChartColumn },
			{ id: 'editor', label: 'Editor', icon: StickyNote },
			{ id: 'cover', label: 'Cover', icon: ImageGlyph },
			{ id: 'document', label: 'Document', icon: FileGlyph }
		]
	}
];

const sectionIds = sections.flatMap((g) => g.items.map((i) => i.id));

function useTheme() {
	const [dark, setDark] = useState(() => {
		try {
			return localStorage.getItem('theme') === 'dark';
		} catch {
			return false;
		}
	});
	useEffect(() => {
		document.documentElement.dataset.theme = dark ? 'dark' : 'light';
		try {
			localStorage.setItem('theme', dark ? 'dark' : 'light');
		} catch {
			/* a private window */
		}
	}, [dark]);
	return [dark, setDark] as const;
}

/* The browser reports which sections cross a line a quarter of the way
   down the viewport; the open one is the last of those in page order.
   Nothing is measured on scroll. */
function useScrollSpy(ids: string[], root: React.RefObject<HTMLElement | null>) {
	const [active, setActive] = useState(ids[0]);
	useEffect(() => {
		const el = root.current;
		if (!el) return;
		const above = new Set<string>();
		const io = new IntersectionObserver(
			(entries) => {
				for (const e of entries) {
					const past =
						e.boundingClientRect.top <
						(e.rootBounds?.top ?? 0) + (e.rootBounds?.height ?? 0) * 0.25;
					if (past) above.add(e.target.id);
					else above.delete(e.target.id);
				}
				let current = ids[0];
				for (const id of ids) if (above.has(id)) current = id;
				setActive(current);
			},
			{ root: el, rootMargin: '-25% 0px 0px 0px', threshold: 0 }
		);
		for (const id of ids) {
			const s = document.getElementById(id);
			if (s) io.observe(s);
		}
		return () => io.disconnect();
	}, [ids, root]);
	return [active, setActive] as const;
}

function Section({
	id,
	title,
	text,
	children
}: {
	id: string;
	title: string;
	text: string;
	children: React.ReactNode;
}) {
	return (
		<section id={id} className="section">
			<div className="section-head">
				<h2 className="section-title">{title}</h2>
				<p className="section-text">{text}</p>
			</div>
			{children}
		</section>
	);
}

/* A component on a seam, with a switch between its wide and compact
   widths; the seam still drags. */
function Fold({
	id,
	wide,
	compact,
	min = 240,
	max = 1400,
	snap,
	note,
	children
}: {
	id: string;
	wide: number;
	compact: number;
	min?: number;
	max?: number;
	snap?: Snap;
	note: string;
	children: React.ReactNode;
}) {
	const [width, setWidth] = useState(wide);
	const mode = width <= compact ? 'compact' : 'wide';
	return (
		<div className="fold-wrap">
			<div className="fold-tools">
				<Segmented
					value={mode}
					onChange={(m) => setWidth(m === 'compact' ? compact : wide)}
					tabs={[
						{ id: 'wide', label: 'Wide' },
						{ id: 'compact', label: 'Compact' }
					]}
				/>
			</div>
			<Split
				id={id}
				width={width}
				onWidth={setWidth}
				min={min}
				max={max}
				snap={snap}
				className="fold"
			>
				{children}
				<div className="fold-note">{note}</div>
			</Split>
		</div>
	);
}

const markdownSample = `## What changed

The note on the claim now carries **who** wrote it and *when*, and both go back to Sikka in the same write.

- \`Claim.svelte\` stamps the note before it is sent
- the plan record reads the fee schedule from the payer, not the practice
- a claim without a note is still a valid claim

| Practice | Notes today | Written back |
| --- | --- | --- |
| Miramar | 12 | 12 |
| Pembroke | 4 | 0 |

> Pembroke has no claim note in Sikka, so nothing is written there.

Run it with \`make up\` and open [the claims desk](https://dev.marzy.com) to see it.`;

function Specimen({
	label,
	column,
	plain,
	children
}: {
	label: string;
	column?: boolean;
	plain?: boolean;
	children: React.ReactNode;
}) {
	return (
		<div className="specimen">
			<div className="specimen-label">{label}</div>
			<div
				className={['stage', column && 'stage-column', plain && 'stage-plain']
					.filter(Boolean)
					.join(' ')}
			>
				{children}
			</div>
		</div>
	);
}

const scales = ['gray', 'azure', 'blue', 'green', 'amber', 'red', 'pink', 'purple', 'teal'];
const stops = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950];
const roles = [
	'background',
	'background-subtle',
	'muted',
	'border',
	'foreground',
	'foreground-secondary',
	'muted-foreground',
	'primary',
	'success',
	'warning',
	'danger',
	'info'
];
const spaces = [1, 2, 3, 4, 5, 6, 8, 10, 12, 16];
const hues: Hue[] = ['blue', 'green', 'amber', 'red', 'pink', 'purple', 'teal', 'navy', 'gray'];

const agentNames: Record<string, string> = {
	claude: 'Claude Code',
	codex: 'Codex',
	gemini: 'Gemini'
};
const modelNames: Record<string, string> = { opus: 'Opus', sonnet: 'Sonnet', fable: 'Fable' };
const effortNames: Record<string, string> = {
	low: 'Low',
	medium: 'Medium',
	high: 'High',
	max: 'Max'
};

const usageRows = [
	{
		id: 'five_hour',
		label: 'Session',
		value: 0.21,
		resets: resetsIn(new Date(Date.now() + 2 * 3600000).toISOString())
	},
	{
		id: 'seven_day',
		label: 'Week',
		value: 0.29,
		resets: resetsIn(new Date(Date.now() + 5 * 86400000).toISOString())
	},
	{
		id: 'seven_day_opus',
		label: 'Week, Opus',
		value: 0.88,
		resets: resetsIn(new Date(Date.now() + 5 * 86400000).toISOString())
	}
];

const menuItems: MenuEntry[] = [
	{ id: 'rename', label: 'Rename', icon: Pencil },
	{ id: 'duplicate', label: 'Duplicate', icon: Copy, hint: '⌘D' },
	{ id: 'link', label: 'Copy link', icon: Link },
	{ divider: true },
	{ id: 'archive', label: 'Archive', icon: Archive },
	{ id: 'delete', label: 'Delete', icon: Trash2, danger: true }
];

interface Person {
	id: string;
	name: string;
	role: string;
	client: string;
	hours: number;
	status: string;
}

const people: Person[] = [
	{
		id: '1',
		name: 'Kaisey Ahmed',
		role: 'Payroll',
		client: 'Lazar & Co',
		hours: 31.5,
		status: 'Active'
	},
	{
		id: '2',
		name: 'Ian Reyes',
		role: 'Accounting',
		client: 'Lazar & Co',
		hours: 12,
		status: 'Active'
	},
	{
		id: '3',
		name: 'Yolanda Price',
		role: 'Front desk',
		client: 'Horizon Dental',
		hours: 40,
		status: 'Away'
	},
	{ id: '4', name: 'Lulu Chen', role: 'Product', client: 'Lulu', hours: 8.25, status: 'Invited' }
];

const columns: Column<Person>[] = [
	{ id: 'name', label: 'Name', render: (p) => p.name },
	{ id: 'role', label: 'Role', render: (p) => p.role },
	{ id: 'client', label: 'Client', render: (p) => p.client },
	{
		id: 'status',
		label: 'Status',
		render: (p) => (
			<Badge hue={p.status === 'Active' ? 'green' : p.status === 'Away' ? 'amber' : 'gray'}>
				{p.status}
			</Badge>
		)
	},
	{ id: 'hours', label: 'Hours', align: 'end', render: (p) => p.hours.toFixed(2) }
];

interface Turn {
	id: number;
	role: 'user' | 'agent';
	text?: string;
	steps?: Array<{ label: string; detail?: string }>;
	pending?: boolean;
}

const opening: Turn[] = [
	{ id: 1, role: 'user', text: 'Run Medical Valley for the 5th.' },
	{
		id: 2,
		role: 'agent',
		steps: [
			{ label: 'Searched the mailbox', detail: 'Angie, Tuesday' },
			{ label: 'Read the time sheet', detail: 'medical-valley-aug.xlsx' }
		],
		text: 'Angie sent hours for eleven people. Two are new since July: Rosa Martinez at 32 hours and Devin Lo at 18. The sheet is filled in on the right; nothing looks off against last period.'
	}
];

const sheetColumns: SheetColumn[] = [
	{ id: 'name', label: 'Employee', width: 180, readonly: true },
	{ id: 'regular', label: 'Regular', kind: 'number', width: 96, placeholder: '0' },
	{ id: 'overtime', label: 'Overtime', kind: 'number', width: 96, placeholder: '0' },
	{ id: 'sick', label: 'Sick', kind: 'number', width: 96, placeholder: '0' },
	{ id: 'rate', label: 'Rate', kind: 'number', width: 96 },
	{ id: 'note', label: 'Note', width: 220, placeholder: 'Anything the client said' },
	{ id: 'department', label: 'Department', width: 140 },
	{ id: 'manager', label: 'Manager', width: 140 }
];

const firstNames = [
	'Angie',
	'Rosa',
	'Devin',
	'Marcus',
	'Priya',
	'Howard',
	'Lena',
	'Tomas',
	'Grace',
	'Omar',
	'June',
	'Felix'
];
const lastNames = [
	'Torres',
	'Martinez',
	'Lo',
	'Bell',
	'Nair',
	'Kim',
	'Okafor',
	'Reyes',
	'Lund',
	'Haddad',
	'Park',
	'Weiss'
];
const departments = ['Front desk', 'Hygiene', 'Billing', 'Assisting', 'Lab'];
const openingRows: SheetRow[] = Array.from({ length: 240 }, (_, i) => ({
	id: String(i + 1),
	values: {
		name: `${firstNames[i % firstNames.length]} ${lastNames[(i * 7) % lastNames.length]}`,
		regular: String(64 + ((i * 13) % 17)),
		overtime: i % 5 === 0 ? String((i % 4) + 0.5) : '',
		sick: i % 9 === 0 ? '8' : '',
		rate: String(18 + (i % 12)),
		note:
			i === 1
				? 'Started August 12, first full period'
				: i === 3
					? 'Out Thursday and Friday, doctor confirmed'
					: '',
		department: departments[i % departments.length],
		manager: i % 3 === 0 ? 'Angie Torres' : ''
	}
}));

const palette = [
	{ id: 'paper', color: 'var(--paper)', label: 'Paper' },
	{ id: 'ink', color: 'var(--ink-500)', label: 'Ink' },
	{ id: 'red', color: 'var(--red-500)', label: 'Red' },
	{ id: 'amber', color: 'var(--amber-500)', label: 'Amber' },
	{ id: 'sky', color: 'var(--blue-200)', label: 'Sky' },
	{ id: 'purple', color: 'var(--purple-500)', label: 'Purple' },
	{ id: 'teal', color: 'var(--teal-500)', label: 'Teal' },
	{ id: 'green', color: 'var(--green-600)', label: 'Green' }
];

const jobsStart: Job[] = [
	{
		id: '1',
		title: 'Medical Valley, September 5',
		verb: 'Writing',
		detail: 'pay-sheet.xlsx',
		state: 'running'
	},
	{
		id: '2',
		title: 'Redwood Tile punch cards',
		verb: 'Reading',
		detail: 'Taking in the mail',
		state: 'running'
	},
	{
		id: '3',
		title: 'Swiss Auto weekly split',
		verb: 'Reading',
		detail: 'Taking in the mail',
		state: 'queued'
	},
	{
		id: '4',
		title: 'Oksana DDS hours',
		verb: 'Reading',
		detail: 'Taking in the mail',
		state: 'queued'
	}
];

/* The demo's jobs move on their own: a running one finishes, a queued one
   starts, so the marks are seen changing. */
function useLiveJobs() {
	const [jobs, setJobs] = useState<Job[]>(jobsStart);
	useEffect(() => {
		const t = window.setInterval(() => {
			setJobs((js) => {
				const running = js.findIndex((j) => j.state === 'running');
				const queued = js.findIndex((j) => j.state === 'queued');
				if (running < 0 && queued < 0) return jobsStart;
				return js.map((j, i) =>
					i === running
						? {
								...j,
								state: 'done',
								verb: 'Wrote',
								detail: j.detail?.replace('Taking in the mail', 'hours.xlsx')
							}
						: i === queued
							? { ...j, state: 'running' }
							: j
				);
			});
		}, 2600);
		return () => window.clearInterval(t);
	}, []);
	return jobs;
}

interface Deal extends BoardCard {
	name: string;
	town: string;
	acv: string;
	owner: { hue: Hue; name: string };
}

const stages = [
	{ id: 'developing', label: 'Developing' },
	{ id: 'early', label: 'Early' },
	{ id: 'strong', label: 'Strong' },
	{ id: 'stalled', label: 'Stalled' },
	{ id: 'won', label: 'Won' }
];

const dealsStart: Deal[] = [
	{
		id: 'd1',
		lane: 'developing',
		name: 'Bright Smile Family Dental',
		town: 'Fresno',
		acv: '10k to 25k',
		owner: { hue: 'green', name: 'Marijn' }
	},
	{
		id: 'd2',
		lane: 'developing',
		name: 'Lazar and Co',
		town: 'Los Angeles',
		acv: 'Under 10k',
		owner: { hue: 'purple', name: 'Lucie' }
	},
	{
		id: 'd3',
		lane: 'early',
		name: 'Redwood Tile',
		town: 'Oakland',
		acv: 'Under 10k',
		owner: { hue: 'green', name: 'Marijn' }
	},
	{
		id: 'd4',
		lane: 'early',
		name: 'Sunrise Orthodontics',
		town: 'San Jose',
		acv: '25k to 50k',
		owner: { hue: 'teal', name: 'Kaisey' }
	},
	{
		id: 'd5',
		lane: 'strong',
		name: 'Horizon Dental',
		town: 'Sacramento',
		acv: '25k to 50k',
		owner: { hue: 'green', name: 'Marijn' }
	},
	{
		id: 'd6',
		lane: 'strong',
		name: 'Lulu',
		town: 'New York',
		acv: 'Over 50k',
		owner: { hue: 'purple', name: 'Lucie' }
	},
	{
		id: 'd7',
		lane: 'stalled',
		name: 'Miramar Dental Group',
		town: 'San Diego',
		acv: '10k to 25k',
		owner: { hue: 'teal', name: 'Kaisey' }
	},
	{
		id: 'd8',
		lane: 'won',
		name: 'Pembroke Family Dentistry',
		town: 'Pembroke Pines',
		acv: '25k to 50k',
		owner: { hue: 'green', name: 'Marijn' }
	}
];

const fdeStages: Stage[] = [
	{ id: 'triaging', label: 'Triaging' },
	{ id: 'designing', label: 'Designing' },
	{ id: 'building', label: 'Building' },
	{ id: 'demo', label: 'Demo' },
	{ id: 'contract', label: 'Contract' },
	{ id: 'live', label: 'Live' }
];

const engagementsStart: Engagement[] = [
	{
		id: 'e1',
		name: 'Horizon Dental',
		town: 'Sacramento',
		stage: 'demo',
		iteration: 3,
		started: '2026-05-12',
		crew: ['Marijn', 'Lucie'],
		hue: 'green'
	},
	{
		id: 'e2',
		name: 'Lulu and Georgia',
		town: 'New York',
		stage: 'demo',
		iteration: 2,
		started: '2026-06-20',
		crew: ['Nim'],
		hue: 'purple'
	},
	{
		id: 'e3',
		name: 'Lazar and Co',
		town: 'Los Angeles',
		stage: 'building',
		started: '2026-08-11',
		crew: ['Lucie', 'Marijn'],
		hue: 'purple'
	},
	{
		id: 'e4',
		name: 'Medical Valley',
		town: 'Modesto',
		stage: 'live',
		started: '2025-11-03',
		crew: ['Marijn'],
		hue: 'green'
	},
	{
		id: 'e5',
		name: 'Sunrise Orthodontics',
		town: 'San Jose',
		stage: 'designing',
		started: '2026-08-24',
		crew: ['Kaisey'],
		hue: 'teal'
	},
	{
		id: 'e6',
		name: 'Miramar Dental Group',
		town: 'San Diego',
		stage: 'contract',
		started: '2026-04-02',
		crew: ['Kaisey', 'Marijn'],
		hue: 'teal'
	},
	{
		id: 'e7',
		name: 'Redwood Tile',
		town: 'Oakland',
		stage: 'triaging',
		started: '2026-09-01',
		crew: ['Marijn'],
		hue: 'green'
	},
	{
		id: 'e8',
		name: 'Swiss Auto',
		town: 'Zurich',
		stage: 'building',
		iteration: 2,
		started: '2026-07-15',
		crew: ['Nim', 'Lucie'],
		hue: 'blue'
	}
];

/* The same stage, stood in once, twice, three times, four: the pass we
   are on cuts the rectangle. */
const passRows: Engagement[] = [
	{
		id: 'p1',
		name: 'Redwood Tile',
		town: 'Oakland',
		stage: 'demo',
		started: '2026-08-24',
		crew: ['Marijn'],
		hue: 'green'
	},
	{
		id: 'p2',
		name: 'Lulu and Georgia',
		town: 'New York',
		stage: 'demo',
		iteration: 2,
		started: '2026-06-20',
		crew: ['Nim'],
		hue: 'purple'
	},
	{
		id: 'p3',
		name: 'Horizon Dental',
		town: 'Sacramento',
		stage: 'demo',
		iteration: 3,
		started: '2026-05-12',
		crew: ['Marijn'],
		hue: 'green'
	},
	{
		id: 'p4',
		name: 'Miramar Dental Group',
		town: 'San Diego',
		stage: 'demo',
		iteration: 4,
		started: '2026-04-02',
		crew: ['Kaisey'],
		hue: 'teal'
	}
];

/* Every unit the span falls into, from the day it began to years on. */
const spanRows: Engagement[] = [
	{
		id: 's1',
		name: 'Flare Apparel',
		stage: 'triaging',
		started: '2026-09-04',
		crew: ['Marijn'],
		hue: 'amber'
	},
	{
		id: 's2',
		name: 'Fieldwork Supply',
		stage: 'triaging',
		started: '2026-09-03',
		crew: ['Marijn'],
		hue: 'amber'
	},
	{
		id: 's3',
		name: 'Redwood Tile',
		stage: 'designing',
		started: '2026-08-28',
		crew: ['Lucie'],
		hue: 'green'
	},
	{
		id: 's4',
		name: 'Lazar and Co',
		stage: 'building',
		started: '2026-08-11',
		crew: ['Lucie'],
		hue: 'purple'
	},
	{
		id: 's5',
		name: 'Horizon Dental',
		stage: 'demo',
		iteration: 3,
		started: '2026-05-12',
		crew: ['Marijn'],
		hue: 'green'
	},
	{
		id: 's6',
		name: 'Medical Valley',
		stage: 'live',
		started: '2025-08-30',
		crew: ['Marijn'],
		hue: 'green'
	},
	{ id: 's7', name: 'Swiss Auto', stage: 'live', started: '2023-06-01', crew: ['Nim'], hue: 'blue' }
];

/* One on it, or four. Each name stands on its own line. */
const crewRows: Engagement[] = [
	{
		id: 'c1',
		name: 'Redwood Tile',
		town: 'Oakland',
		stage: 'designing',
		started: '2026-08-24',
		crew: ['Marijn'],
		hue: 'green'
	},
	{
		id: 'c2',
		name: 'Lazar and Co',
		town: 'Los Angeles',
		stage: 'building',
		started: '2026-08-11',
		crew: ['Lucie Loubet', 'Marijn'],
		hue: 'purple'
	},
	{
		id: 'c3',
		name: 'Horizon Dental',
		town: 'Sacramento',
		stage: 'demo',
		iteration: 3,
		started: '2026-05-12',
		crew: ['Marijn', 'Lucie Loubet', 'Kaisey Ahmed'],
		hue: 'green'
	},
	{
		id: 'c4',
		name: 'Bright Smile Family Dental',
		town: 'Fresno',
		stage: 'demo',
		iteration: 2,
		started: '2026-06-02',
		crew: ['Nim', 'Marijn', 'Kaisey Ahmed', 'Lucie Loubet'],
		hue: 'blue'
	}
];

interface Deploy {
	id: string;
	app: string;
	step: string;
	steps: number;
	at: number;
	value: number;
	tone: 'running' | 'done' | 'failed';
	elapsed: number;
}

const deploysStart: Deploy[] = [
	{
		id: 'dental',
		app: 'Dental',
		step: 'Build',
		steps: 5,
		at: 3,
		value: 0.41,
		tone: 'running',
		elapsed: 743
	},
	{
		id: 'design',
		app: 'Design',
		step: 'Deployed',
		steps: 4,
		at: 4,
		value: 1,
		tone: 'done',
		elapsed: 212
	},
	{
		id: 'schema',
		app: 'Schema',
		step: 'Apply',
		steps: 3,
		at: 2,
		value: 0.66,
		tone: 'failed',
		elapsed: 98
	}
];

/* The demo's running deploy creeps forward, finishes, then starts over. */
function useLiveDeploys() {
	const [rows, setRows] = useState<Deploy[]>(deploysStart);
	useEffect(() => {
		const t = window.setInterval(() => {
			setRows((rs) =>
				rs.map((r) => {
					if (r.tone !== 'running') return r;
					const value = r.value + 0.03;
					if (value >= 1) return { ...r, value: 1, tone: 'done', step: 'Deployed', at: r.steps };
					return {
						...r,
						value,
						elapsed: r.elapsed + 1,
						at: Math.min(r.steps, 1 + Math.floor(value * r.steps))
					};
				})
			);
		}, 900);
		const reset = window.setInterval(() => setRows(deploysStart), 36000);
		return () => {
			window.clearInterval(t);
			window.clearInterval(reset);
		};
	}, []);
	return rows;
}

function clock(seconds: number) {
	const m = Math.floor(seconds / 60);
	const s = seconds % 60;
	return `${m}m ${s}s`;
}

const meetings: CalendarEvent[] = [
	{
		id: 'm1',
		date: '2026-09-02',
		title: 'Kaisey and Ian on payroll',
		time: '2:00 pm',
		hue: 'teal'
	},
	{
		id: 'm2',
		date: '2026-09-04',
		title: 'Claims desk demo with Horizon',
		time: '11:00 am',
		hue: 'green'
	},
	{
		id: 'm3',
		date: '2026-09-04',
		title: 'Lulu on the workbook export',
		time: '4:30 pm',
		hue: 'purple'
	},
	{
		id: 'm4',
		date: '2026-09-08',
		title: 'Redwood Tile punch cards',
		time: '10:00 am',
		hue: 'amber'
	},
	{
		id: 'm5',
		date: '2026-09-11',
		title: 'Sunrise Orthodontics intro',
		time: '1:00 pm',
		hue: 'blue'
	},
	{
		id: 'm6',
		date: '2026-09-12',
		title: 'Plan record review with Yolanda',
		time: '9:30 am',
		hue: 'green'
	},
	{ id: 'm7', date: '2026-09-17', title: 'Miramar follow up', time: '3:00 pm', hue: 'red' },
	{ id: 'm8', date: '2026-09-23', title: 'Pembroke quarterly', time: '11:00 am', hue: 'navy' },
	{ id: 'm9', date: '2026-09-30', title: 'Lazar month close', time: '2:00 pm', hue: 'teal' }
];

const story: Entry[] = [
	{
		id: 'e1',
		name: 'main',
		summary:
			'Payroll reads a client’s history instead of chatting it, and writes the mail in front of it.',
		meta: 'Today'
	},
	{
		id: 'e2',
		name: 'design/system',
		branch: true,
		hue: 'teal',
		summary:
			'One component library for the company, the dessin palette, a CRM record, a board, a calendar, and the day at a glance.',
		meta: 'Today, 41 commits, open'
	},
	{
		id: 'e3',
		name: 'main',
		summary: 'Claude Code’s skills live in the repo, not in a session hook.',
		meta: 'Yesterday'
	},
	{
		id: 'e4',
		name: 'dental/claims-desk',
		branch: true,
		merged: true,
		hue: 'purple',
		summary:
			'The claims queue, a claim page with stamped notes that write back to Sikka, and the plan record.',
		meta: 'Two days ago, 18 commits, merged'
	},
	{
		id: 'e5',
		name: 'payroll/desk',
		branch: true,
		merged: true,
		hue: 'amber',
		summary:
			'A pay page per client driven by the schedule, with the agent on the left and the sheet on the right.',
		meta: 'Three days ago, 12 commits, merged'
	},
	{
		id: 'e6',
		name: 'main',
		summary: 'A client firm cannot exist without a pay schedule.',
		meta: 'Last week'
	}
];

interface CodeTurn {
	id: number;
	role: 'user' | 'agent';
	text?: string;
	pending?: boolean;
	stopped?: boolean;
	meta?: string;
	steps?: Array<{ kind: 'edit' | 'run' | 'search'; title: string; detail?: string }>;
}

const codeStart: CodeTurn[] = [
	{
		id: 1,
		role: 'user',
		text: 'The loader in the login card should be one line, not a spinner with text under it.'
	},
	{
		id: 2,
		role: 'agent',
		text: 'One line now: the medium spinner with the words beside it, where the two buttons were.',
		meta: '1m 12s, 41k tokens',
		steps: [
			{ kind: 'search', title: 'Found the busy state', detail: '2 files' },
			{
				kind: 'edit',
				title: 'Laid the spinner and the words in one row',
				detail: 'Login.css, +4 −6'
			},
			{ kind: 'edit', title: 'Took the spinner down a size', detail: 'Login.tsx, +1 −1' },
			{ kind: 'run', title: 'Ran the type check', detail: 'Passed' }
		]
	}
];

const commands = [
	{ id: 'triage', label: 'Triage', hint: 'after a meeting, decide what to build' },
	{ id: 'align', label: 'Align', hint: 'check the work against the ask' },
	{ id: 'notify', label: 'Notify', hint: 'draft the client an update' },
	{ id: 'meet', label: 'Meet', hint: 'put the next call on the calendar' },
	{ id: 'design', label: 'Design', hint: 'building or changing an interface' },
	{ id: 'compact', label: 'Compact', hint: 'fold the context' }
];

const talksStart: Talk[] = [
	{
		id: 't1',
		channel: 'gmail',
		people: ['Angie Torres'],
		title: 'Hours for the 5th',
		last: 'Attached, two new hires this period. Rosa started the 12th.',
		when: '9:14 am',
		client: 'Medical Valley',
		unread: true
	},
	{
		id: 't2',
		channel: 'slack',
		people: ['Yolanda Price', 'Ian Reyes'],
		title: 'Claims desk',
		last: 'The note went into Sikka, I can see it on the claim.',
		when: '8:52 am',
		client: 'Horizon Dental',
		unread: true
	},
	{
		id: 't3',
		channel: 'imessage',
		people: ['Kaisey Ahmed'],
		title: 'Friday demo',
		last: 'Can we push to next week? Ian is out Friday.',
		when: 'Yesterday',
		client: 'Lazar and Co'
	},
	{
		id: 't4',
		channel: 'linkedin',
		people: ['Sara Sugarman Brenner'],
		title: 'Intro from Marcus',
		last: 'Would love to see what you did for the vendor sheets.',
		when: 'Yesterday',
		client: 'Lulu and Georgia'
	},
	{
		id: 't5',
		channel: 'discord',
		people: ['Nim', 'Lucie Loubet'],
		title: 'Design system',
		last: 'The wave loader is so good',
		when: 'Tuesday'
	},
	{
		id: 't6',
		channel: 'whatsapp',
		people: ['Howard Kim'],
		title: 'Weekly split',
		last: 'Same as last week, thanks.',
		when: 'Tuesday',
		client: 'Swiss Auto'
	},
	{
		id: 't7',
		channel: 'outlook',
		people: ['Aaron Califf'],
		title: 'Eligibility before the patient sits down',
		last: 'Front desk says it is already saving them the afternoon.',
		when: 'Monday',
		client: 'Horizon Dental'
	}
];

const saidStart: Said[] = [
	{
		id: 's1',
		from: 'Angie Torres',
		channel: 'gmail',
		when: 'Tuesday, 4:02 pm',
		text: 'Hi Marijn, hours for the 5th are attached. Two new hires this period, Rosa Martinez and Devin Lo. Rosa started the 12th so she is a partial.',
		attachments: ['medical-valley-aug.xlsx']
	},
	{
		id: 's2',
		from: 'Marijn',
		channel: 'gmail',
		when: 'Tuesday, 4:40 pm',
		text: 'Got it, thanks Angie. Devin shows 18 hours and 2.5 overtime, is that right for a first period?',
		mine: true
	},
	{
		id: 's3',
		from: 'Angie Torres',
		channel: 'imessage',
		when: 'Wednesday, 8:10 am',
		text: 'Yes he covered Marcus on Thursday. Marcus was out Thursday and Friday, doctor confirmed.'
	},
	{
		id: 's4',
		from: 'Angie Torres',
		channel: 'gmail',
		when: 'Today, 9:14 am',
		text: 'Attached, two new hires this period. Rosa started the 12th. Let me know once it is approved and I will tell them.',
		attachments: ['medical-valley-aug-v2.xlsx']
	}
];

const channels: ChannelId[] = [
	'gmail',
	'outlook',
	'slack',
	'discord',
	'linkedin',
	'imessage',
	'whatsapp',
	'phone'
];

const callStart: Participant[] = [
	{ id: 'me', name: 'Marijn', me: true, camera: true },
	{ id: 'k', name: 'Kaisey Ahmed', speaking: true, camera: true },
	{ id: 'i', name: 'Ian Reyes', muted: true }
];

const targetsStart: Target[] = [
	{
		id: 'a1',
		name: 'Halo Dental Partners',
		stage: 'Identified',
		score: 93,
		why: 'Clear fit at the right moment. Twelve locations on one practice management system, a new billing lead in March, and Amy knows their CTO from Sikka.',
		fit: 88,
		timing: 97,
		connection: 94,
		connections: ['Amy Chen'],
		vertical: 'Dental'
	},
	{
		id: 'a2',
		name: 'Lune Orthodontics',
		stage: 'Identified',
		score: 92,
		why: 'Growing fast and hiring a controller. No one there yet.',
		fit: 90,
		timing: 92,
		connection: 40,
		connections: [],
		vertical: 'Dental'
	},
	{
		id: 'a3',
		name: 'Brightcomm Accounting',
		stage: 'Identified',
		score: 91,
		why: 'Payroll for forty firms, the shape Lazar had. Ian can introduce.',
		fit: 95,
		timing: 78,
		connection: 90,
		connections: ['Ian Reyes'],
		vertical: 'Accounting'
	},
	{
		id: 'a4',
		name: 'Orbit Home Goods',
		stage: 'Identified',
		score: 90,
		fit: 82,
		timing: 88,
		connection: 72,
		connections: ['Sara Sugarman Brenner'],
		vertical: 'Ecommerce'
	},
	{
		id: 'a5',
		name: 'Velocity Distribution',
		stage: 'In motion',
		score: 89,
		fit: 84,
		timing: 90,
		connection: 80,
		connections: ['Marcus Bell', 'Howard Kim'],
		vertical: 'Distribution'
	},
	{
		id: 'a6',
		name: 'Compass Family Dentistry',
		stage: 'In motion',
		score: 88,
		fit: 86,
		timing: 80,
		connection: 88,
		connections: ['Yolanda Price'],
		vertical: 'Dental'
	},
	{
		id: 'a7',
		name: 'Terminal Logistics',
		stage: 'In motion',
		score: 88,
		fit: 80,
		timing: 92,
		connection: 60,
		connections: ['Priya Nair', 'Devin Lo'],
		vertical: 'Distribution'
	},
	{
		id: 'a8',
		name: 'Bolt Payroll Services',
		stage: 'In motion',
		score: 87,
		fit: 92,
		timing: 70,
		connection: 74,
		connections: ['Kaisey Ahmed'],
		vertical: 'Accounting'
	},
	{
		id: 'a9',
		name: 'Pulse Pediatric Dental',
		stage: 'In motion',
		score: 86,
		fit: 78,
		timing: 84,
		connection: 82,
		connections: ['Aaron Califf', 'Yolanda Price'],
		vertical: 'Dental'
	},
	{
		id: 'a10',
		name: 'Fieldwork Supply',
		stage: 'Queued',
		score: 78,
		fit: 70,
		timing: 76,
		connection: 58,
		connections: ['Lena Okafor'],
		vertical: 'Distribution'
	},
	{
		id: 'a11',
		name: 'SyncSpace',
		stage: 'Queued',
		score: 69,
		fit: 60,
		timing: 72,
		connection: 40,
		connections: [],
		vertical: 'Software'
	},
	{
		id: 'a12',
		name: 'Flare Apparel',
		stage: 'Queued',
		score: 64,
		fit: 66,
		timing: 58,
		connection: 50,
		connections: ['Tomas Reyes', 'Lucie Loubet'],
		vertical: 'Ecommerce'
	},
	{
		id: 'a13',
		name: 'Safeguard Benefits',
		stage: 'Queued',
		score: 62,
		fit: 58,
		timing: 64,
		connection: 30,
		connections: [],
		vertical: 'Insurance'
	}
];
const targetStages = ['Identified', 'In motion', 'Queued'];

const docStart = `# Horizon Dental, the claims desk

What we learned on the call of **September 2**, and what we build next.

## What Yolanda asked for

- The queue should show what is *stuck*, not everything.
- A note on a claim has to land in Sikka, or it is written twice.
- The plan record should carry the payer's real fee schedule.

## Next

- [x] The claims queue, sorted by what blocks payment
- [x] A claim page with stamped notes that write back
- [ ] The plan record with the fee schedule
- [ ] Eligibility before the patient sits down

> Every correction makes it sharper. Ian signs; Yolanda decides whether it gets used.

The demo is on the [12th](https://marzy.com), then the plan record.
`;

const documentSample = `One page. The loop we run, the machinery that makes two engineers ship like twenty. The factory is its own repo and GCP project; production codebases are its **output**, never its home.

## The Loop

1. **Land** talk to a client in industry X. Agree on a design-partner deal. At least one product designer and CEO on the first call.
2. **Triage** after the meeting, use a triage skill to build a demo in one session.
3. **Ship** every session ends in production, and the client hears about it the same day.

> The demo is the contract.
`;

/* Pinned above the rail, so it holds its place while the sections scroll. */
const shellPinned: SidebarItem[] = [{ id: 'search', label: 'Search', icon: Search, hint: '⌘K' }];

const shellSections: SidebarSection[] = [
	{
		id: 'desk',
		items: [
			{ id: 'inbox', label: 'Inbox', icon: InboxGlyph, marked: true },
			{ id: 'coding', label: 'Coding', icon: Hammer },
			{ id: 'docs', label: 'Docs', icon: FileGlyph }
		]
	},
	{
		id: 'company',
		label: 'Company',
		items: [
			{ id: 'crm', label: 'CRM', icon: Store },
			{ id: 'calls', label: 'Calls', icon: Video }
		]
	}
];
const shellCommands: CommandItem[] = [
	{ id: 'inbox', label: 'Inbox', group: 'Go to', icon: InboxGlyph },
	{ id: 'crm', label: 'CRM', group: 'Go to', icon: Store },
	{ id: 'coding', label: 'Coding', group: 'Go to', icon: Hammer },
	{ id: 'triage', label: 'Triage a client', hint: 'after a meeting', group: 'Skills' },
	{ id: 'notify', label: 'Notify a client', hint: 'draft an update', group: 'Skills' },
	{ id: 'new-client', label: 'New client', group: 'Create', icon: Plus }
];

function Page() {
	const toast = useToast();
	const [loading, setLoading] = useState(false);
	const [checks, setChecks] = useState({ a: false, b: true, c: true });
	const [radio, setRadio] = useState('weekly');
	const [on, setOn] = useState(true);
	const [off, setOff] = useState(false);
	const [selected, setSelected] = useState<string | null>(null);
	const [tab, setTab] = useState('all');
	const [segment, setSegment] = useState('list');
	const [progress, setProgress] = useState(0.35);
	const [dialog, setDialog] = useState(false);
	const [motionOn, setMotionOn] = useState(false);
	const [docEmoji, setDocEmoji] = useState('🏭');
	const [docTitle, setDocTitle] = useState('Software Factory Playbook');
	const [docBody, setDocBody] = useState(documentSample);
	const [lastVerb, setLastVerb] = useState('');
	const [streamed, setStreamed] = useState('');
	const streamTimer = useRef<number | undefined>(undefined);
	const replay = () => {
		window.clearInterval(streamTimer.current);
		let at = 0;
		setStreamed('');
		streamTimer.current = window.setInterval(
			() => {
				at = Math.min(markdownSample.length, at + 4 + Math.floor(Math.random() * 40));
				setStreamed(markdownSample.slice(0, at));
				if (at >= markdownSample.length) window.clearInterval(streamTimer.current);
			},
			90 + Math.floor(Math.random() * 160)
		);
	};
	const [pageUrl, setPageUrl] = useState('https://marzy.com');
	const [draft, setDraft] = useState('');
	const [attachments, setAttachments] = useState([{ id: 'a', name: 'medical-valley-aug.xlsx' }]);
	const [turns, setTurns] = useState<Turn[]>(opening);
	const [sending, setSending] = useState(false);
	const [sheetRows, setSheetRows] = useState<SheetRow[]>(openingRows);
	const seq = useRef(10);
	const [tool, setTool] = useState('shapes');
	const [swatch, setSwatch] = useState('paper');
	const [role, setRole] = useState<string | null>('editor');
	const [access, setAccess] = useState('viewer');
	const [prompt, setPrompt] = useState('');
	const jobsNow = useLiveJobs();
	const [deals, setDeals] = useState<Deal[]>(dealsStart);
	const [engagements, setEngagements] = useState<Engagement[]>(engagementsStart);
	const deploys = useLiveDeploys();
	const [loginStep, setLoginStep] = useState<LoginStep>('door');
	const [agent, setAgent] = useState<string | null>('claude');
	const [model, setModel] = useState<string | null>('opus');
	const [effort, setEffort] = useState<string | null>('high');
	const [asked, setAsked] = useState<'allowed' | 'always' | 'denied' | undefined>(undefined);
	const [ask2, setAsk2] = useState<'allowed' | 'always' | 'denied' | undefined>(undefined);
	const [codeDraft, setCodeDraft] = useState('');
	const [shellArea, setShellArea] = useState('inbox');
	const [shellOpen, setShellOpen] = useState(false);
	const [doc, setDoc] = useState(docStart);
	const [mailDraft, setMailDraft] = useState<Draft>({
		from: 'marijn',
		to: [{ id: 'sarah', name: 'Sarah Chen', email: 'sarah@lune.com' }],
		cc: [],
		bcc: [],
		subject: 'Your point about forecast accuracy',
		body: 'Hi Sarah,\n\nGreat chatting at the conference. You mentioned your forecast was off by 40% last quarter and you could not figure out why until it was too late.\n\nThat is exactly what we catch: deals that sound healthy but show early warning signs in the conversation data. Happy to show you what it would have looked like on your pipeline.\n\nWorth a look?'
	});
	const [mailSending, setMailSending] = useState(false);
	const [mailFiles, setMailFiles] = useState<Array<{ id: string; name: string }>>([]);
	const [targetTab, setTargetTab] = useState('all');
	const shownTargets = targetsStart.filter((t) => targetTab === 'all' || t.stage === targetTab);
	const [callPeople, setCallPeople] = useState<Participant[]>(callStart);
	const [mic, setMic] = useState(true);
	const [cam, setCam] = useState(true);
	const [sharing, setSharing] = useState(false);
	const [callChatOpen, setCallChatOpen] = useState(true);
	const [callLine, setCallLine] = useState('');
	const [callLines, setCallLines] = useState([
		{ id: 'l1', from: 'Kaisey Ahmed', when: '2:03 pm', text: 'Sharing the sheet in a sec' },
		{ id: 'l2', from: 'Ian Reyes', when: '2:04 pm', text: 'Can you zoom in on the overtime column' }
	]);
	const guests = [
		'Yolanda Price',
		'Angie Torres',
		'Howard Kim',
		'Lucie Loubet',
		'Nim',
		'Rosa Martinez'
	];
	const [talks, setTalks] = useState<Talk[]>(talksStart);
	const [talk, setTalk] = useState('t1');
	const [talkOpened, setTalkOpened] = useState(false);
	const [talkFilter, setTalkFilter] = useState('all');
	const [talkQuery, setTalkQuery] = useState('');
	const [reply, setReply] = useState('');
	const [replyVia, setReplyVia] = useState<string | null>('gmail');
	const shownTalks = talks.filter(
		(t) =>
			(talkFilter === 'all' || (talkFilter === 'unread' ? t.unread : t.channel === talkFilter)) &&
			(!talkQuery ||
				`${t.people.join(' ')} ${t.title} ${t.last} ${t.client ?? ''}`
					.toLowerCase()
					.includes(talkQuery.toLowerCase()))
	);
	const openTalk = (id: string) => {
		setTalkOpened(true);
		setTalk(id);
		setTalks((ts) => ts.map((t) => (t.id === id ? { ...t, unread: false } : t)));
	};
	const current = talks.find((t) => t.id === talk) ?? talks[0];
	const [codeTurns, setCodeTurns] = useState<CodeTurn[]>(codeStart);
	const [working, setWorking] = useState(false);
	const [queue, setQueue] = useState<Array<{ id: string; text: string }>>([]);
	const timer = useRef<number | undefined>(undefined);
	const startWork = (id: number) => {
		setWorking(true);
		window.clearTimeout(timer.current);
		timer.current = window.setTimeout(() => {
			setCodeTurns((t) =>
				t.map((x) =>
					x.id === id
						? {
								...x,
								pending: false,
								text: 'Done. The change is on the branch.',
								meta: '38s, 12k tokens',
								steps: [
									{ kind: 'edit', title: 'Made the change', detail: '+6 −2' },
									{ kind: 'run', title: 'Ran the type check', detail: 'Passed' }
								]
							}
						: x
				)
			);
			setWorking(false);
		}, 5000);
	};
	const codeSend = () => {
		const text = codeDraft.trim();
		if (!text) return;
		const id = ++codeSeq.current;
		setCodeDraft('');
		if (working) {
			setQueue((q) => [...q, { id: String(id), text }]);
			return;
		}
		setCodeTurns((t) => [
			...t,
			{ id, role: 'user', text },
			{ id: id + 1, role: 'agent', pending: true }
		]);
		startWork(id + 1);
	};
	const codeStop = () => {
		window.clearTimeout(timer.current);
		setWorking(false);
		setCodeTurns((t) =>
			t.map((x) =>
				x.pending
					? { ...x, pending: false, text: 'Stopped.', meta: 'Interrupted after 12s', stopped: true }
					: x
			)
		);
	};
	useEffect(() => {
		if (working || queue.length === 0) return;
		const [next, ...rest] = queue;
		const id = ++codeSeq.current;
		setQueue(rest);
		setCodeTurns((t) => [
			...t,
			{ id, role: 'user', text: next.text },
			{ id: id + 1, role: 'agent', pending: true }
		]);
		startWork(id + 1);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [working, queue]);
	const codeSeq = useRef(20);
	const [codeAttachments, setCodeAttachments] = useState<Attached[]>([]);
	const rehearsed: SessionView = {
		id: 'design',
		repo: 'marzy',
		cwd: '/Users/marijn/marzy',
		branch: 'design/system',
		sessionId: 'rehearsed',
		agent,
		setAgent,
		model,
		models: [
			{ value: 'opus', label: 'Opus' },
			{ value: 'sonnet', label: 'Sonnet' },
			{ value: 'fable', label: 'Fable' }
		],
		setModel,
		effort,
		efforts: [
			{ value: 'low', label: 'Low' },
			{ value: 'medium', label: 'Medium' },
			{ value: 'high', label: 'High' },
			{ value: 'max', label: 'Max' }
		],
		setEffort,
		context: 0.42,
		draft: codeDraft,
		setDraft: setCodeDraft,
		turns: codeTurns,
		working,
		queue,
		commands,
		attachments: codeAttachments,
		attach: (paths) =>
			setCodeAttachments((a) => [
				...a,
				...paths.map((p) => ({ id: p, name: p.split('/').pop() ?? p, path: p }))
			]),
		unattach: (id) => setCodeAttachments((a) => a.filter((x) => x.id !== id)),
		canFork: true,
		fork: async () => null,
		send: codeSend,
		stop: codeStop,
		clear: () => setCodeTurns([]),
		unqueue: (id) => setQueue((q) => q.filter((x) => x.id !== id)),
		answer: () => toast('Answered')
	};
	const [cursor, setCursor] = useState(() => new Date(2026, 8, 4));
	const [calView, setCalView] = useState<CalendarView>('month');
	const [loginBusy, setLoginBusy] = useState(false);
	const signIn = () => {
		setLoginBusy(true);
		window.setTimeout(() => {
			setLoginBusy(false);
			setLoginStep('workspaces');
		}, 1200);
	};
	const [todos, setTodos] = useState({ a: false, b: false, c: true });
	const [meterValue, setMeterValue] = useState(0.41);
	const [client, setClient] = useState({
		name: 'Horizon Dental',
		town: 'Sacramento',
		website: 'horizondental.com',
		phone: '(916) 555 0142',
		vertical: 'dental',
		stage: 'strong',
		acv: '25k to 50k',
		owner: 'marijn',
		lastTouch: '2026-09-02',
		notes:
			'Two locations, Yolanda runs the front desk at both. They came in through Sikka and want claims first, eligibility second. Ian signs; Yolanda decides whether it gets used.\n\nNext: the claims desk demo on the 12th, then the plan record.'
	});
	const edit = (key: keyof typeof client) => (value: string | null) =>
		setClient((c) => ({ ...c, [key]: value ?? '' }));
	const doneCount = jobsNow.filter((j) => j.state === 'done').length;

	const send = () => {
		const text = draft.trim();
		if (!text) return;
		const id = ++seq.current;
		setTurns((t) => [
			...t,
			{ id, role: 'user', text },
			{ id: id + 1, role: 'agent', pending: true }
		]);
		setDraft('');
		setSending(true);
		window.setTimeout(() => {
			setTurns((t) =>
				t.map((x) =>
					x.id === id + 1
						? {
								id: x.id,
								role: 'agent',
								steps: [{ label: 'Checked the sheet' }],
								text: 'Done. Nothing else changed, so the sheet stands as it is.'
							}
						: x
				)
			);
			setSending(false);
		}, 1800);
	};

	const load = () => {
		setLoading(true);
		window.setTimeout(() => setLoading(false), 2200);
	};

	return (
		<div className="page">
			<div className="page-intro">
				<h1 className="page-title">Components</h1>
				<p className="page-lede">
					One system for Mulholland and Marzy: the sites, the sign in, the mail, and every app's
					desk. Light, tokens all the way down, and nothing a component states for itself.
				</p>
				<div className="row">
					<Button
						variant="primary"
						trailing={ArrowRight}
						onClick={() => window.open('/app.html', '_blank')}
					>
						Open the app
					</Button>
				</div>
			</div>

			<Section
				id="color"
				title="Color"
				text="Nine scales at eleven stops, then roles on top. Components only ever name a role. The blue does the acting; the other hues are for canvases, groups and tags, each a tint to sit on, a middle tone to draw with and a deep tone to write with."
			>
				<Specimen label="Scales" plain>
					<div className="scales">
						{scales.map((s) => (
							<div key={s} className="scale-row">
								<span className="scale-name">{s}</span>
								<span className="scale-stops">
									{stops.map((n) => (
										<Tooltip key={n} label={`${s} ${n}`}>
											<span className="stop" style={{ background: `var(--${s}-${n})` }} />
										</Tooltip>
									))}
								</span>
							</div>
						))}
					</div>
				</Specimen>
				<Specimen label="Roles" plain>
					<div className="roles">
						{roles.map((r) => (
							<div key={r} className="role">
								<span className="stop" style={{ background: `var(--${r})` }} />
								<span className="role-name">{r.replace(/-/g, ' ')}</span>
							</div>
						))}
					</div>
				</Specimen>
			</Section>

			<Section
				id="typography"
				title="Typography"
				text="Notion's stack: the system sans, regular and medium, bold only for a page title, no tracking. Nearly everything is the base size; a title is the one exception. Emoji come from the emoji face, never the text one."
			>
				<Specimen label="Sizes" plain>
					<div className="type-rows">
						<div className="type-row">
							<span className="type-note">Display</span>
							<span className="type-2xl">Payroll, done by Friday</span>
						</div>
						<div className="type-row">
							<span className="type-note">Title</span>
							<span className="type-xl">Payroll, done by Friday</span>
						</div>
						<div className="type-row">
							<span className="type-note">Lead</span>
							<span className="type-lg">Every client, every schedule, one desk.</span>
						</div>
						<div className="type-row">
							<span className="type-note">Base</span>
							<span className="type-base">
								Every client, every schedule, one desk. Approve the sheet and the mail goes out.
							</span>
						</div>
						<div className="type-row">
							<span className="type-note">Medium</span>
							<span className="type-base type-medium">Every client, every schedule, one desk.</span>
						</div>
						<div className="type-row">
							<span className="type-note">Secondary</span>
							<span className="type-sm">Sent to Angie at Medical Valley on Tuesday.</span>
						</div>
						<div className="type-row">
							<span className="type-note">Small</span>
							<span className="type-xs">Updated a minute ago</span>
						</div>
					</div>
				</Specimen>
			</Section>

			<Section
				id="spacing"
				title="Spacing"
				text="A four pixel step. Inside a container the distance to every edge is the same value, and siblings share one gap."
			>
				<Specimen label="Steps" plain>
					<div className="spaces">
						{spaces.map((n) => (
							<div key={n} className="space-row">
								<span className="space-name">{n * 4}</span>
								<span className="space-bar" style={{ width: `var(--space-${n})` }} />
							</div>
						))}
					</div>
				</Specimen>
			</Section>

			<Section
				id="motion"
				title="Motion"
				text="Three durations and three curves. Fast for a tone shift, normal for anything that moves, slow for anything that arrives, and a spring for a pane that gives a little past its limits and springs back when let go. Nothing scales under the pointer or the finger: a hover and a press answer in tone, at once, because a control that has to finish moving before it looks pressed is a control that feels slow."
			>
				<Specimen label="Durations" plain>
					<div className="motion-rows">
						{(['fast', 'normal', 'slow', 'spring'] as const).map((d) => (
							<div
								key={d}
								className={['motion-row', motionOn && 'motion-on'].filter(Boolean).join(' ')}
							>
								<span className="motion-name">{d}</span>
								<span className="motion-track">
									<span className={`motion-ball motion-${d}`} />
								</span>
							</div>
						))}
						<div className="row">
							<Button onClick={() => setMotionOn((m) => !m)}>Play</Button>
						</div>
					</div>
				</Specimen>
				<Specimen label="Press and lift">
					<Button variant="primary">Press me</Button>
					<IconButton icon={Paperclip} label="Attach" />
					<IconButton icon={Plus} label="New" />
					<Checkbox
						label="Squish"
						checked={checks.a}
						onChange={(e) => setChecks({ ...checks, a: e.target.checked })}
					/>
					<button type="button" className="sidebar-user chrome" onClick={() => toast('You')}>
						<Avatar name="Marijn" size="sm" />
						<span className="sidebar-user-name">Marijn</span>
					</button>
				</Specimen>
			</Section>

			<Section
				id="icons"
				title="Icons"
				text="Lucide, one stroke width, at three sizes: small in dense rows, the default in controls, large where an icon leads a message."
			>
				<Specimen label="Sizes">
					<div className="icon-row">
						{[InboxIcon, Calendar, Users, Folder, FileText, Mail, Settings, Star].map((I, i) => (
							<Icon key={i} icon={I} size="sm" />
						))}
					</div>
					<div className="icon-row">
						{[InboxIcon, Calendar, Users, Folder, FileText, Mail, Settings, Star].map((I, i) => (
							<Icon key={i} icon={I} />
						))}
					</div>
					<div className="icon-row">
						{[InboxIcon, Calendar, Users, Folder, FileText, Mail, Settings, Star].map((I, i) => (
							<Icon key={i} icon={I} size="lg" />
						))}
					</div>
				</Specimen>
				<Specimen label="The emoji a page can wear">
					<span className="icon-row">
						<Emoji>🏭</Emoji>
						<Emoji>🎯</Emoji>
						<Emoji>🔬</Emoji>
						<Emoji>🦷</Emoji>
						<Emoji size="sm">📄</Emoji>
						<Emoji size="lg">🧠</Emoji>
					</span>
				</Specimen>
				<Specimen label="Picking one">
					<EmojiPicker
						value={docEmoji}
						onChange={setDocEmoji}
						trigger={<Button leading={<Emoji size="sm">{docEmoji}</Emoji>}>Emoji</Button>}
					/>
				</Specimen>
			</Section>

			<Section
				id="button"
				title="Button"
				text="A filled one for the act, a quieter one for the rest. One primary a screen. Loading swaps the icon for the spinner and keeps the width."
			>
				<Specimen label="Variants">
					<Button variant="primary" icon={Plus}>
						New client
					</Button>
					<Button>Secondary</Button>
					<Button variant="outline">Outline</Button>
					<Button variant="ghost">Ghost</Button>
					<Button variant="destructive" icon={Trash2}>
						Delete
					</Button>
				</Specimen>
				<Specimen label="Sizes">
					<Button variant="primary" size="sm">
						Small
					</Button>
					<Button variant="primary">Medium</Button>
					<Button variant="primary" size="lg">
						Large
					</Button>
				</Specimen>
				<Specimen label="States">
					<Button variant="primary" loading={loading} onClick={load}>
						Approve payroll
					</Button>
					<Button loading={loading} onClick={load}>
						Send
					</Button>
					<Button variant="primary" disabled>
						Disabled
					</Button>
					<Button disabled>Disabled</Button>
				</Specimen>
			</Section>

			<Section
				id="icon-button"
				title="Icon button"
				text="Only an icon, with its name in a tooltip and for the screen reader."
			>
				<Specimen label="Variants">
					<Tooltip label="Filter">
						<IconButton icon={Filter} label="Filter" />
					</Tooltip>
					<Tooltip label="Edit">
						<IconButton icon={Pencil} label="Edit" />
					</Tooltip>
					<Tooltip label="Star">
						<IconButton icon={Star} label="Star" variant="secondary" />
					</Tooltip>
					<Tooltip label="Add">
						<IconButton icon={Plus} label="Add" variant="primary" />
					</Tooltip>
					<Tooltip label="Delete">
						<IconButton icon={Trash2} label="Delete" variant="outline" />
					</Tooltip>
				</Specimen>
				<Specimen label="With an icon">
					<IconButton icon={Paperclip} label="Attach" />
					<IconButton icon={GitFork} label="Fork" />
					<IconButton icon={Eraser} label="Start over" size="sm" />
				</Specimen>
			</Section>

			<Section
				id="menu"
				title="Menu"
				text="Verbs under a trigger. The arrow keys walk it, Enter runs one, and the dangerous one sits last in red."
			>
				<Specimen label="Dropdown">
					<Menu
						trigger={<Button trailing={ChevronsUpDown}>Actions</Button>}
						items={menuItems}
						onSelect={setLastVerb}
					/>
					<Menu
						trigger={<IconButton icon={Settings} label="More" />}
						items={menuItems}
						onSelect={setLastVerb}
						placement="bottom-end"
					/>
					{lastVerb && <span className="status-line">You chose {lastVerb}</span>}
				</Specimen>
				<Specimen label="Choices, with submenus">
					<Menu
						stay
						trigger={
							<Button trailing={ChevronsUpDown} icon={Bot}>
								{[agentNames[agent ?? ''], modelNames[model ?? ''], effortNames[effort ?? '']]
									.filter(Boolean)
									.join(', ')}
							</Button>
						}
						items={[
							{
								id: 'agent',
								label: 'Agent',
								icon: Bot,
								hint: agentNames[agent ?? ''],
								items: Object.entries(agentNames).map(([v, l]) => ({
									id: `agent:${v}`,
									label: l,
									checked: v === agent
								}))
							},
							{
								id: 'model',
								label: 'Model',
								icon: Cpu,
								hint: modelNames[model ?? ''],
								items: Object.entries(modelNames).map(([v, l]) => ({
									id: `model:${v}`,
									label: l,
									checked: v === model
								}))
							},
							{
								id: 'effort',
								label: 'Effort',
								icon: Gauge,
								hint: effortNames[effort ?? ''],
								items: Object.entries(effortNames).map(([v, l]) => ({
									id: `effort:${v}`,
									label: l,
									checked: v === effort
								}))
							}
						]}
						onSelect={(id) => {
							const [what, value] = id.split(':');
							if (what === 'agent') setAgent(value);
							else if (what === 'model') setModel(value);
							else if (what === 'effort') setEffort(value);
						}}
					/>
				</Specimen>
			</Section>

			<Section
				id="input"
				title="Input"
				text="A hairline box that turns blue when you are in it. The label sits above, the hint or the error below."
			>
				<Specimen label="Fields" column>
					<div className="row-fields">
						<Field label="Firm" hint="As it appears on the pay stub.">
							<Input placeholder="Medical Valley" />
						</Field>
						<Field label="Contact" error="An address needs an at sign.">
							<Input defaultValue="angie.medicalvalley" invalid />
						</Field>
					</div>
					<div className="row-fields">
						<Field label="Search">
							<Input icon={Search} placeholder="Find a client" />
						</Field>
						<Field label="Disabled">
							<Input placeholder="Not now" disabled />
						</Field>
					</div>
					<Field label="Note to the client">
						<Textarea placeholder="Anything they should know before Friday." />
					</Field>
				</Specimen>
				<Specimen label="Sizes">
					<Input size="sm" placeholder="Small" className="half" />
					<Input placeholder="Medium" className="half" />
					<Input size="lg" placeholder="Large" className="half" />
				</Specimen>
			</Section>

			<Section
				id="select"
				title="Select"
				text="Our own list, never the platform's. The face is an input, the list is a menu, and the current option carries a check."
			>
				<Specimen label="Schedule">
					<Field label="Pay schedule">
						<Select
							value={selected}
							onChange={setSelected}
							placeholder="Choose a schedule"
							options={[
								{ value: 'weekly', label: 'Weekly', hint: 'Fridays' },
								{ value: 'biweekly', label: 'Every two weeks', hint: 'Alternate Fridays' },
								{ value: 'semimonthly', label: 'Twice a month', hint: '15th and last' },
								{ value: 'monthly', label: 'Monthly', hint: 'Last business day' }
							]}
						/>
					</Field>
				</Specimen>
			</Section>

			<Section
				id="checkbox"
				title="Checkbox"
				text="The check draws itself in. A mixed state is a bar."
			>
				<Specimen label="States">
					<Checkbox
						label="Off"
						checked={checks.a}
						onChange={(e) => setChecks({ ...checks, a: e.target.checked })}
					/>
					<Checkbox
						label="On"
						checked={checks.b}
						onChange={(e) => setChecks({ ...checks, b: e.target.checked })}
					/>
					<Checkbox
						label="Some"
						indeterminate
						checked={checks.c}
						onChange={(e) => setChecks({ ...checks, c: e.target.checked })}
					/>
					<Checkbox label="Disabled" disabled />
				</Specimen>
			</Section>

			<Section id="radio" title="Radio" text="One of a few, with room for a line under each.">
				<Specimen label="Options">
					<RadioGroup
						name="cadence"
						value={radio}
						onChange={setRadio}
						options={[
							{ value: 'weekly', label: 'Weekly', hint: 'Runs every Friday' },
							{ value: 'biweekly', label: 'Every two weeks', hint: 'Runs on alternate Fridays' },
							{ value: 'monthly', label: 'Monthly', hint: 'Runs on the last business day' }
						]}
					/>
				</Specimen>
			</Section>

			<Section id="switch" title="Switch" text="A thumb that slides across. Blue when it is on.">
				<Specimen label="States">
					<Switch checked={on} onChange={setOn} label="Mail the client" />
					<Switch checked={off} onChange={setOff} label="Auto approve" />
					<Switch checked disabled onChange={() => {}} label="Locked" />
				</Specimen>
			</Section>

			<Section id="tabs" title="Tabs" text="One underline that travels between them.">
				<Specimen label="Tabs" plain>
					<Tabs
						value={tab}
						onChange={setTab}
						tabs={[
							{ id: 'all', label: 'All' },
							{ id: 'open', label: 'Open' },
							{ id: 'done', label: 'Done' },
							{ id: 'archived', label: 'Archived' }
						]}
					/>
				</Specimen>
			</Section>

			<Section
				id="segmented"
				title="Segmented"
				text="The same choice, as a thumb sliding in a track. For views, not pages."
			>
				<Specimen label="Views">
					<Segmented
						value={segment}
						onChange={setSegment}
						tabs={[
							{ id: 'list', label: 'List' },
							{ id: 'board', label: 'Board' },
							{ id: 'calendar', label: 'Calendar' }
						]}
					/>
				</Specimen>
			</Section>

			<Section
				id="breadcrumb"
				title="Breadcrumb"
				text="Where the page sits. The last crumb is the page itself and does the work of a title."
			>
				<Specimen label="Trail">
					<Breadcrumb
						items={[
							{ label: 'Clients', icon: Store, href: '#' },
							{ label: 'Medical Valley', icon: Hospital, href: '#' },
							{ label: 'September 5' }
						]}
					/>
				</Specimen>
			</Section>

			<Section
				id="sidebar"
				title="Sidebar"
				text="What every app wears on the left, and what this page wears too. Sections of rows from the top, a head only where there is a workspace to switch, as on this page, each an icon and a name, the open one on a tint. A mark on a row where something waits. Search is the first row, its shortcut at the end. At the foot you, the plan and settings. No hairline, no shadow; a tone shift is enough."
			>
				<Specimen label="Sections">
					<div className="rail-stage">
						<Sidebar
							sections={shellSections}
							pinned={shellPinned}
							active={shellArea}
							onSelect={(id) => (id === 'search' ? setShellOpen(true) : setShellArea(id))}
							footer={
								<SidebarFoot
									usage={<Usage rows={usageRows} />}
									user="Marijn"
									menu={[
										{ heading: 'marijn@mulholland.ai' },
										{ id: 'standing', label: 'Mulholland', icon: Store, disabled: true },
										{ divider: true },
										{ id: 'signout', label: 'Sign out', icon: LogOut, danger: true }
									]}
									onMenu={(id) => toast(id === 'signout' ? 'Signed out' : id)}
									onSettings={() => toast('Settings')}
								/>
							}
						/>
					</div>
				</Specimen>
				<Specimen label="The frame" column plain>
					<div className="rail-stage">
						<Shell
							sidebar={
								<Sidebar sections={shellSections} active={shellArea} onSelect={setShellArea} />
							}
							chat={<div className="fold-note">The chat column, on its seam.</div>}
							crumbs={[{ label: 'Inbox', icon: InboxGlyph }, { label: 'Angie Torres' }]}
						>
							<div className="fold-note">The work. Drag the seam between.</div>
						</Shell>
					</div>
				</Specimen>
			</Section>

			<Section
				id="spinner"
				title="Spinner"
				text="A short stretch of the progress bar's wave, rolling in place, in the colour of whatever it sits in. One language for work happening, the same as the bar."
			>
				<Specimen label="Sizes">
					<Spinner size="sm" />
					<Spinner size="md" />
					<Spinner size="lg" />
				</Specimen>
				<Specimen label="In place">
					<Button variant="primary" loading>
						Sending
					</Button>
					<Button loading>Working</Button>
					<span
						className="row"
						style={{ gap: 'var(--control-gap)', color: 'var(--foreground-secondary)' }}
					>
						<Spinner size="sm" />
						Reading the mailbox
					</span>
				</Specimen>
			</Section>

			<Section
				id="progress"
				title="Progress"
				text="How far along, as a share of one. The filled part is a wave that keeps rolling while there is still work, the rest a flat line with a small gap before it. It settles flat and green when it is done."
			>
				<Specimen label="Wave" column>
					<Progress value={progress} label="Payrolls approved" />
					<Progress value={0.15} label="Starting" />
					<Progress value={1} label="Done" />
					<Progress value={progress} wave={false} label="Flat" />
					<div className="row">
						<Button onClick={() => setProgress((p) => Math.min(1, p + 0.2))}>Advance</Button>
						<Button variant="ghost" onClick={() => setProgress(0.1)}>
							Reset
						</Button>
					</div>
				</Specimen>
			</Section>

			<Section
				id="badge"
				title="Badge"
				text="A word on a tint, low and close, as Notion's tags are. The word carries the meaning; there is no dot. An icon can lead it."
			>
				<Specimen label="Hues">
					{hues.map((h) => (
						<Badge key={h} hue={h}>
							{h[0].toUpperCase() + h.slice(1)}
						</Badge>
					))}
				</Specimen>
				<Specimen label="With an icon">
					<Badge hue="green" icon={CircleCheck}>
						Approved
					</Badge>
					<Badge hue="amber" icon={Hourglass}>
						Waiting on client
					</Badge>
					<Badge hue="red" icon={Ban}>
						Rejected
					</Badge>
				</Specimen>
				<Specimen label="With an icon">
					<Badge hue="green" icon={Sparkles}>
						Approved
					</Badge>
					<Badge hue="amber" icon={Bell}>
						Waiting on client
					</Badge>
					<Badge hue="red" icon={Trash2}>
						Rejected
					</Badge>
				</Specimen>
			</Section>

			<Section
				id="callout"
				title="Callout"
				text="A line the page wants read, an icon leading it as a Notion callout does. Each tone brings its own; give it another where one says it better."
			>
				<Specimen label="Tones" column plain>
					<Callout tone="info" title="Heads up">
						The September run moves a day earlier for Labor Day.
					</Callout>
					<Callout tone="success">The sheet was sent to Angie and she approved it.</Callout>
					<Callout tone="warning" title="Missing hours">
						Two clock outs are missing on Redwood Tile's punch cards.
					</Callout>
					<Callout tone="danger" title="Not sent">
						Gmail turned the mail away. Try again in a minute.
					</Callout>
				</Specimen>
				<Specimen label="Its own icon" column plain>
					<Callout icon={Stethoscope} title="Horizon Dental">
						The claims desk goes live on the 12th.
					</Callout>
				</Specimen>
			</Section>

			<Section
				id="tooltip"
				title="Tooltip"
				text="A word or two after a beat. Never the only place a label lives."
			>
				<Specimen label="Hover">
					<Tooltip label="Above">
						<Button>Above</Button>
					</Tooltip>
					<Tooltip label="Below" placement="bottom">
						<Button>Below</Button>
					</Tooltip>
					<Tooltip label="To the right" placement="right-start">
						<Button>Right</Button>
					</Tooltip>
				</Specimen>
			</Section>

			<Section
				id="toast"
				title="Toast"
				text="A line at the bottom right that leaves on its own. Newest last."
			>
				<Specimen label="Fire one">
					<Button onClick={() => toast('Saved')}>Plain</Button>
					<Button onClick={() => toast('Payroll sent to Angie', { tone: 'success' })}>
						Success
					</Button>
					<Button onClick={() => toast('Could not reach Gmail', { tone: 'danger' })}>Danger</Button>
					<Button
						onClick={() =>
							toast('Client archived', {
								action: { label: 'Undo', onClick: () => toast('Restored', { tone: 'success' }) }
							})
						}
					>
						With undo
					</Button>
				</Specimen>
			</Section>

			<Section
				id="empty"
				title="Empty"
				text="Nothing here yet, said plainly, with the one thing to do about it."
			>
				<Specimen label="No clients" plain>
					<Empty
						icon={InboxIcon}
						title="No payrolls this week"
						action={
							<Button variant="primary" icon={Plus}>
								Add a client
							</Button>
						}
					>
						When a client's schedule lands on a week, their payroll shows here.
					</Empty>
				</Specimen>
			</Section>

			<Section
				id="composer"
				title="Composer"
				text="Where you talk to the agent. It grows with what you type, Enter sends, Shift and Enter break the line, and what you attached sits above the words, and the book keeps the words as a skill to run again by name. While the agent works the send is a stop, and what you send waits above the field until its turn, where you can still take it back. A slash at the start opens the commands."
			>
				<Specimen label="Working" column>
					<Composer
						value=""
						onChange={() => {}}
						onSend={() => {}}
						onStop={() => toast('Stopped')}
						working
						commands={commands}
						queued={[{ id: 'q1', text: 'Then run the tests again' }]}
						onUnqueue={() => toast('Taken out of the queue')}
					/>
				</Specimen>
				<Specimen label="Empty" column>
					<Composer
						value={draft}
						onChange={setDraft}
						onSend={send}
						onAttach={() =>
							setAttachments((a) => [...a, { id: String(Date.now()), name: 'punch-cards.pdf' }])
						}
						attachments={attachments}
						onRemoveAttachment={(id) => setAttachments((a) => a.filter((x) => x.id !== id))}
						sending={sending}
						placeholder="Ask about this payroll"
					/>
				</Specimen>
			</Section>

			<Section
				id="message"
				title="Message"
				text="One turn. Your words sit in a tint on the right; the agent's are plain text on the page, with the verbs it ran above them as quiet lines."
			>
				<Specimen label="Turns" column>
					<Message role="user" author="Marijn" time="2:14 pm">
						Run Medical Valley for the 5th.
					</Message>
					<Message
						role="agent"
						author="Marzy"
						time="2:14 pm"
						onCopy={() => toast('Copied')}
						onRetry={() => toast('Trying again')}
					>
						<Step icon={MailOpen} label="Searched the mailbox" detail="Angie, Tuesday" />
						<Step
							icon={FileText}
							label="Reading the time sheet"
							detail="medical-valley-aug.xlsx"
							state="running"
						/>
						<span>Angie sent hours for eleven people. Two are new since July.</span>
					</Message>
					<Message role="agent" author="Marzy" pending />
				</Specimen>
			</Section>

			<Section
				id="thread"
				title="Thread"
				text="The turns in order, the newest at the bottom, the composer under them. Send something and the agent answers."
			>
				<Specimen label="Conversation" column>
					<Thread count={turns.length}>
						{turns.map((t) => (
							<Message
								key={t.id}
								role={t.role}
								pending={t.pending}
								onCopy={t.role === 'agent' ? () => toast('Copied') : undefined}
							>
								{t.steps?.map((st, i) => (
									<Step
										key={i}
										icon={i === 0 ? MailOpen : FileText}
										label={st.label}
										detail={st.detail}
									/>
								))}
								{t.text && <span>{t.text}</span>}
							</Message>
						))}
					</Thread>
					<Composer
						value={draft}
						onChange={setDraft}
						onSend={send}
						sending={sending}
						placeholder="Reply"
					/>
				</Specimen>
			</Section>

			<Section
				id="markdown"
				title="Markdown"
				text="What the agent writes, with its shape kept: headings at the base size and a heavier weight, lists, quotes, tables, and links that open elsewhere. Code is a tint in the same face as everything else, never another face. Streaming, the words come in at an even pace whatever the bursts they arrived in."
			>
				<Specimen label="An agent's answer" column plain>
					<Markdown>{markdownSample}</Markdown>
				</Specimen>
				<Specimen label="Arriving in bursts, shown at an even pace" column plain>
					<div className="row">
						<Button size="sm" onClick={replay}>
							{streamed.length && streamed.length < markdownSample.length ? 'Again' : 'Stream it'}
						</Button>
					</div>
					<Markdown stream>{streamed}</Markdown>
				</Specimen>
			</Section>

			<Section
				id="header"
				title="Header"
				text="Where you are, and what you can do here. A title or a trail on the left, the acts on the right. It sits at the top of the main column, under nothing."
			>
				<Specimen label="Title" column plain>
					<Header
						title="Clients"
						actions={
							<>
								<Button variant="primary" size="sm" icon={Plus}>
									New client
								</Button>
								<IconButton icon={Search} label="Search" />
							</>
						}
					/>
					<Header
						crumbs={
							<Breadcrumb items={[{ label: 'Clients', href: '#' }, { label: 'Horizon Dental' }]} />
						}
						actions={<IconButton icon={Search} label="Search" />}
					/>
				</Specimen>
			</Section>

			<Section
				id="commands"
				title="Commands"
				text="Everything from the keyboard, shaped as Notion's search: a bare line to type in, then rows with their icon. Cmd K opens it; type to narrow, arrows to move, Enter to run. Grouped by what a thing is: a place to go, a skill to run, a thing to create."
			>
				<Specimen label="Palette">
					<Button onClick={() => setShellOpen(true)}>Open the palette</Button>
					<Commands
						open={shellOpen}
						onClose={() => setShellOpen(false)}
						items={shellCommands}
						onRun={(id) => toast(`Ran ${id}`)}
					/>
				</Specimen>
			</Section>

			<Section
				id="split"
				title="Split"
				text="Two panes on a seam that can be dragged. The first pane has the width, the second takes the rest; the width is kept under the seam's id between pages and launches, and a double click puts it back. Every pane is a container, so what is inside folds to its compact form by its own width, never by measuring. Drag the seam under the threshold and it snaps to the compact width. A stacking split goes further: under 480 the seam goes and the two panes become pages you go between, with a way back."
			>
				<Specimen label="A list that folds to its people" column plain>
					<Fold
						id="specimen-split"
						wide={360}
						compact={64}
						min={200}
						max={560}
						snap={{ below: 180, to: 64 }}
						note="Drag the seam in. Under 180 the list snaps to 64 and each row is the person with the channel's glyph at their corner."
					>
						<div className="inbox-list">
							<Inbox talks={talks} selected={talk} onSelect={openTalk} />
						</div>
					</Fold>
				</Specimen>
			</Section>

			<Section
				id="browser"
				title="Browser"
				text="A page beside the work: a way back, the address, reload and a way out to the browser at the top, the page below. The agent opens it for the person to see, a change on localhost mostly, and the desk switches to it. Natively the page is a webview laid over the body; here it is a frame."
			>
				<Specimen label="A page the agent opened" column plain>
					<div className="browser-stage">
						<Browser
							url={pageUrl}
							onOpen={setPageUrl}
							onBack={() => toast('Back')}
							onReload={() => toast('Reloaded')}
							onOutside={() => toast('Opened outside')}
						>
							{pageUrl ? (
								<iframe className="browser-frame" src={pageUrl} title="The page" />
							) : undefined}
						</Browser>
					</div>
				</Specimen>
			</Section>

			<Section
				id="usage"
				title="Usage"
				text="How much of the plan is used, under one glyph at the foot of the rail, on hover: a row per window, the session's and the week's, with the meter, the number and when it resets. Near the limit it says so."
			>
				<Specimen label="Every session">
					<div className="rail-stage rail-stage-short">
						<Usage rows={usageRows} />
					</div>
				</Specimen>
			</Section>

			<Section
				id="login"
				title="Login"
				text="The one door for every app, and the page after it. First the two accounts a person can come in with; once they are in, the workspaces they belong to. The card stays put and what is on it crosses over."
			>
				<Specimen label="Sign in">
					<div className="login-stage">
						<Login
							step={loginStep}
							title="Sign in to Marzy"
							busy={loginBusy}
							onGoogle={signIn}
							onMicrosoft={signIn}
							workspaces={[
								{ id: 'mulholland', name: 'Mulholland', app: 'Workbench' },
								{ id: 'horizon', name: 'Horizon Dental', app: 'Dental' },
								{ id: 'lazar', name: 'Lazar and Co', app: 'Accounting' }
							]}
							onPick={(id) => toast(`Into ${id}`)}
							account="marijn@mulholland.ai"
							onSignOut={() => setLoginStep('door')}
						/>
					</div>
				</Specimen>
				<Specimen label="Brand marks">
					<span className="icon-row">
						<Brand name="google" />
						<Brand name="microsoft" />
					</span>
				</Specimen>
			</Section>

			<Section
				id="talks"
				title="Inbox"
				text="Every conversation across every channel in one list, for everyone on the team. Three lines a row: who, with the channel and when at the end; what; and the last thing said, the client after it. Nothing is filled, not on hover and not for the open one; unread is the weight of the words. Filter by channel or search across all of it."
			>
				<Specimen label="All channels" column plain>
					<div className="inbox-tools">
						<Segmented
							value={talkFilter}
							onChange={setTalkFilter}
							tabs={[
								{ id: 'all', label: 'All' },
								{ id: 'unread', label: 'Unread' },
								{ id: 'gmail', label: 'Mail' },
								{ id: 'slack', label: 'Slack' },
								{ id: 'imessage', label: 'Messages' }
							]}
						/>
						<Input
							size="sm"
							icon={Search}
							placeholder="Search everything"
							value={talkQuery}
							onChange={(e) => setTalkQuery(e.target.value)}
							className="inbox-search"
						/>
					</div>
					<Split
						id="specimen-inbox"
						width={360}
						min={200}
						max={560}
						snap={{ below: 180, to: 64 }}
						className="inbox-split"
						stack
						show={talkOpened ? 'rest' : 'first'}
					>
						<div className="inbox-list">
							{shownTalks.length ? (
								<Inbox talks={shownTalks} selected={talk} onSelect={openTalk} />
							) : (
								<Empty icon={InboxIcon} title="Nothing here">
									No conversation matches.
								</Empty>
							)}
						</div>
						<div className="inbox-pane">
							<Conversation
								title={current.title}
								back={() => setTalkOpened(false)}
								people={[...current.people, 'Marijn']}
								client={current.client ? <Badge hue="gray">{current.client}</Badge> : undefined}
								actions={
									<>
										<Tooltip label="Reply">
											<IconButton icon={Reply} label="Reply" size="sm" />
										</Tooltip>
										<Tooltip label="Forward">
											<IconButton icon={Forward} label="Forward" size="sm" />
										</Tooltip>
										<Tooltip label="Archive">
											<IconButton
												icon={Archive}
												label="Archive"
												size="sm"
												onClick={() => toast('Archived')}
											/>
										</Tooltip>
									</>
								}
								said={
									current.id === 't1'
										? saidStart
										: [
												{
													id: 'x',
													from: current.people[0],
													channel: current.channel,
													when: current.when,
													text: current.last
												}
											]
								}
								footer={
									<Composer
										value={reply}
										onChange={setReply}
										onSend={() => {
											toast(`Sent over ${replyVia}`);
											setReply('');
										}}
										onAttach={() => toast('Attach')}
										placeholder={`Reply to ${current.people[0]}`}
										leading={
											<Select
												size="sm"
												value={replyVia}
												onChange={setReplyVia}
												options={[
													{ value: 'gmail', label: 'Reply over Gmail' },
													{ value: 'imessage', label: 'Reply over iMessage' },
													{ value: 'slack', label: 'Reply over Slack' }
												]}
											/>
										}
									/>
								}
							/>
						</div>
					</Split>
				</Specimen>
			</Section>

			<Section
				id="conversation"
				title="Conversation"
				text="One conversation, whole: its title, who is in it, the client it is tagged to, then everything said in order with who, where and when. A thread can cross channels, a mail answered by a text stays one conversation. The reply goes back out the channel you pick."
			>
				<Specimen label="Across channels" column plain>
					<Conversation
						title="Hours for the 5th"
						people={['Angie Torres', 'Marijn']}
						client={<Badge hue="gray">Medical Valley</Badge>}
						said={saidStart}
					/>
				</Specimen>
			</Section>

			<Section
				id="mail"
				title="Mail"
				text="A mail being written. From as a quiet pick of your accounts, To as the people with their marks, Cc and Bcc out of the way until asked for, the subject, then the words as prose. Attach and send later at the foot, Send on the right, lit only when there is someone, a subject and a body."
			>
				<Specimen label="Send email" column plain>
					<Panel
						tone="light"
						className="mail-panel"
						title="Send email"
						onClose={() => toast('Closed')}
					>
						<MailComposer
							draft={mailDraft}
							onChange={setMailDraft}
							accounts={[
								{ value: 'marijn', label: 'marijn@mulholland.ai' },
								{ value: 'hello', label: 'hello@mulholland.ai' }
							]}
							attachments={mailFiles}
							onAttach={() =>
								setMailFiles((f) => [...f, { id: String(Date.now()), name: 'pipeline-review.pdf' }])
							}
							onRemoveAttachment={(id) => setMailFiles((f) => f.filter((x) => x.id !== id))}
							onSchedule={() => toast('Pick a time')}
							sending={mailSending}
							onSend={() => {
								setMailSending(true);
								window.setTimeout(() => {
									setMailSending(false);
									toast(`Sent to ${mailDraft.to.map((p) => p.name).join(', ')}`, {
										tone: 'success'
									});
								}, 1400);
							}}
						/>
					</Panel>
				</Specimen>
			</Section>

			<Section
				id="channel"
				title="Channel"
				text="Where a message came from: a line glyph in the channel's hue, its name on hover, or the word on the channel's tint where there is room for it. Never a logo, never a box; an emoji is for a page a person made."
			>
				<Specimen label="Glyphs">
					{channels.map((c) => (
						<Channel key={c} id={c} />
					))}
				</Specimen>
				<Specimen label="Words">
					{channels.map((c) => (
						<Channel key={c} id={c} word />
					))}
				</Specimen>
			</Section>

			<Section
				id="session"
				title="Session"
				text="A coding session as a column, the desk's own: the head says the repo and the branch, holds the context and the ways to open a folder, fork, and start over; the turns stream in as words, thought, a folded run of steps, a plan and what it asks; the composer at the foot has one picker for the agent, the model and the effort, and the agent's slash commands. What the agent needs answered takes the composer's place."
			>
				<Specimen label="Working with the agent" column plain>
					<Fold
						id="specimen-session"
						wide={720}
						compact={300}
						note="Drag the seam in. Under 360 the repo name goes and the context is its number; under 280 the picker is its glyph and a step is its title."
					>
						<div className="column-stage">
							<AgentColumn
								session={rehearsed}
								agents={agentKinds}
								local
								onOpen={() => toast('Open a repository')}
								onFork={() => toast('Forked')}
								onAttach={() => toast('Attach')}
							/>
						</div>
					</Fold>
				</Specimen>
				<Specimen label="One turn, as it streamed" column plain>
					<Message role="agent">
						<AgentTurn
							turn={{
								id: 1,
								role: 'agent',
								blocks: [
									{
										kind: 'thought',
										text: 'The card had two steps in one cell so it never resized; the fold is a one row grid.',
										seconds: 12
									},
									{
										kind: 'tool',
										id: 't1',
										title: 'Read the login card',
										action: 'search',
										state: 'done',
										detail: 'Login.tsx'
									},
									{
										kind: 'tool',
										id: 't2',
										title: 'Laid the spinner and the words in one row',
										action: 'edit',
										state: 'done',
										detail: 'Login.css'
									},
									{
										kind: 'plan',
										entries: [
											{ id: 'a', text: 'Find the busy state', state: 'done' },
											{ id: 'b', text: 'One row', state: 'done' },
											{ id: 'c', text: 'Type check and commit', state: 'doing' }
										]
									},
									{
										kind: 'ask',
										id: 'ask1',
										title: 'Run git push to origin',
										detail: 'design/system, 2 commits',
										options: [
											{ optionId: 'y', name: 'Allow', kind: 'allow_once' },
											{ optionId: 'a', name: 'Always', kind: 'allow_always' },
											{ optionId: 'n', name: 'Deny', kind: 'reject_once' }
										]
									},
									{
										kind: 'text',
										text: 'One line now: the **medium spinner** with the words beside it. It is on `design/system`.'
									}
								]
							}}
							onAnswer={(_, optionId) => toast(`Answered ${optionId}`)}
						/>
					</Message>
				</Specimen>
				<Specimen label="Asked something" column plain>
					<div className="column-stage">
						<AgentColumn
							session={{
								...rehearsed,
								question: {
									message: 'Which client is this for?',
									fields: [
										{
											key: 'client',
											label: 'Client',
											kind: 'options',
											required: true,
											options: [
												{ value: 'horizon', label: 'Horizon Dental' },
												{ value: 'lulu', label: 'Lulu' }
											]
										}
									],
									answer: (v) => toast(`Chose ${String(v.client)}`),
									decline: () => toast('Declined')
								}
							}}
							local
						/>
					</div>
				</Specimen>
			</Section>

			<Section
				id="actions"
				title="Actions"
				text="What the agent did, one sentence each about what changed, with the file and its counts as the detail. A run in flight wears the spinner; a failure the cross. Several fold to one line. No file is ever opened."
			>
				<Specimen label="Kinds" column>
					<Action
						kind="edit"
						title="Kept the primary look while a button loads"
						detail="Button.css, +12 −3"
					/>
					<Action kind="run" title="Ran the tests" detail="41 passed" />
					<Action kind="run" state="running" title="Building" detail="vite build" />
					<Action
						kind="run"
						state="failed"
						title="Ran the type check"
						detail="1 error, a name clashes with the window"
					/>
					<Action
						kind="search"
						title="Found every place that measures on scroll"
						detail="4 files"
					/>
					<Action
						kind="fetch"
						title="Read the agent protocol spec"
						detail="agentclientprotocol.com"
					/>
					<Action kind="tool" title="Listed the open tasks" detail="marzy" />
				</Specimen>
				<Specimen label="Folded" column>
					<Actions summary="7 steps">
						<Action kind="search" title="Searched for the busy state" detail="2 files" />
						<Action
							kind="edit"
							title="Laid the spinner and the words in one row"
							detail="Login.css, +4 −6"
						/>
						<Action kind="run" title="Ran the type check" detail="Passed" />
					</Actions>
					<Actions summary="3 steps" busy>
						<Action kind="search" title="Searched for the busy state" detail="2 files" />
						<Action
							kind="edit"
							title="Laid the spinner and the words in one row"
							detail="Login.css, +4 −6"
						/>
						<Action kind="run" state="running" title="Running the type check" />
					</Actions>
					<Thinking seconds={12}>
						The card had two steps in one cell so it never resized; the fold is a one row grid.
					</Thinking>
					<Thinking />
				</Specimen>
			</Section>

			<Section
				id="ask"
				title="Ask"
				text="The agent asks before it does something it is not trusted with yet: allow once, always for this session, or not. Answered, it settles to what was decided."
			>
				<Specimen label="Permission" column>
					<Ask
						title="Run git push to origin"
						detail="design/system, 2 commits"
						answered={asked}
						onAllow={() => setAsked('allowed')}
						onAlways={() => setAsked('always')}
						onDeny={() => setAsked('denied')}
					/>
					<Ask
						title="Delete web/design/src/site"
						detail="12 files"
						answered={ask2}
						onAllow={() => setAsk2('allowed')}
						onDeny={() => setAsk2('denied')}
					/>
					<div className="row">
						<Button
							variant="ghost"
							size="sm"
							onClick={() => {
								setAsked(undefined);
								setAsk2(undefined);
							}}
						>
							Ask again
						</Button>
					</div>
				</Specimen>
			</Section>

			<Section
				id="plan"
				title="Plan"
				text="The agent's plan for the turn, each line with where it stands: to do, doing with the spinner, done and struck."
			>
				<Specimen label="Turn" column>
					<Plan
						items={[
							{ id: 'a', text: 'Read the failing test', state: 'done' },
							{ id: 'b', text: 'Move the layer into the flow beside its trigger', state: 'done' },
							{ id: 'c', text: 'Drop the scroll listener', state: 'doing' },
							{ id: 'd', text: 'Run the type check', state: 'todo' },
							{ id: 'e', text: 'Commit on the branch', state: 'todo' }
						]}
					/>
				</Specimen>
			</Section>

			<Section
				id="elicit"
				title="Elicit"
				text="The agent asks before it goes on, in the composer's place: its words, then what it needs as buttons or fields, and a way to decline. A lone choice answers on the press. When the answer happens elsewhere, a sign-in, the link to open sits first."
			>
				<Specimen label="A choice" column plain>
					<Elicit
						message="Which client is this for?"
						fields={[
							{
								key: 'client',
								label: 'Client',
								kind: 'options',
								required: true,
								options: [
									{ value: 'horizon', label: 'Horizon Dental' },
									{ value: 'lulu', label: 'Lulu' },
									{ value: 'lazar', label: 'Lazar and Co' }
								]
							}
						]}
						onAnswer={(v) => toast(`Chose ${String(v.client)}`)}
						onDecline={() => toast('Declined')}
					/>
				</Specimen>
				<Specimen label="A form" column plain>
					<Elicit
						message="Please answer the following questions."
						fields={[
							{
								key: 'sheets',
								label: 'Sheets',
								description: 'Which sheets should the workbook carry?',
								kind: 'multi',
								options: [
									{ value: 'rules', label: 'Rules' },
									{ value: 'versions', label: 'Versions' },
									{ value: 'notes', label: 'Notes' }
								]
							},
							{ key: 'title', label: 'Title', kind: 'text', required: true },
							{ key: 'draft', label: 'As a draft', kind: 'boolean' }
						]}
						onAnswer={(v) => toast(`Answered ${Object.keys(v).join(', ')}`)}
						onDecline={() => toast('Declined')}
					/>
				</Specimen>
				<Specimen label="A sign-in" column plain>
					<Elicit
						message="Sign in to Codex to continue."
						fields={[
							{
								key: 'method',
								label: 'How',
								kind: 'options',
								required: true,
								options: [
									{ value: 'chatgpt', label: 'ChatGPT' },
									{ value: 'apikey', label: 'API key' }
								]
							}
						]}
						open={{ label: 'Open the sign-in', onOpen: () => toast('Opened the browser') }}
						onAnswer={(v) => toast(`Signing in with ${String(v.method)}`)}
						onDecline={() => toast('Declined')}
					/>
				</Specimen>
			</Section>

			<Section
				id="dialog"
				title="Dialog"
				text="A scrim, a panel, and a way out. Escape and the scrim both close it."
			>
				<Specimen label="Modal">
					<Button variant="primary" onClick={() => setDialog(true)}>
						Archive client
					</Button>
					<Dialog
						open={dialog}
						onClose={() => setDialog(false)}
						title="Archive Medical Valley"
						footer={
							<>
								<Button onClick={() => setDialog(false)}>Cancel</Button>
								<Button
									variant="destructive"
									onClick={() => {
										setDialog(false);
										toast('Medical Valley archived', {
											action: { label: 'Undo', onClick: () => {} }
										});
									}}
								>
									Archive
								</Button>
							</>
						}
					>
						<p>
							Their payrolls stop running and their people lose access. Nothing is deleted; you can
							bring them back from the archive.
						</p>
						<Field label="Type the firm's name to confirm">
							<Input placeholder="Medical Valley" />
						</Field>
					</Dialog>
				</Specimen>
			</Section>

			<Section id="popover" title="Popover" text="Any content, anchored to a trigger.">
				<Specimen label="Anchored">
					<Popover trigger={<Button icon={Filter}>Filter</Button>}>
						<div className="column">
							<Checkbox label="Active" defaultChecked />
							<Checkbox label="Away" />
							<Checkbox label="Invited" />
						</div>
					</Popover>
				</Specimen>
			</Section>

			<Section
				id="panel"
				title="Panel"
				text="A dark card floating on a coloured canvas, the house surface. It re-themes every component inside it, so a composer, a select or a list dropped into a panel needs nothing of its own."
			>
				<Specimen label="Create">
					<Panel title="Create something new" onClose={() => toast('Closed')}>
						<Composer
							value={prompt}
							onChange={setPrompt}
							onSend={() => {
								toast('Sent');
								setPrompt('');
							}}
							onAttach={() => toast('Attach')}
							placeholder="Describe what you want"
						/>
						<Tile
							hue="red"
							icon={Layers}
							title="Start with a template"
							onClick={() => toast('Templates')}
						/>
						<Tile
							hue="navy"
							icon={LayoutGrid}
							title="Start with a surface"
							onClick={() => toast('Surfaces')}
						/>
					</Panel>
				</Specimen>
				<Specimen label="Space">
					<Panel title="Space" onClose={() => toast('Closed')}>
						<div className="column" style={{ gap: 0 }}>
							<span>Website revamp, prototypes</span>
							<span className="status-line">7 people</span>
						</div>
						<List>
							{['Lucie Loubet', 'Farhan', 'Ayush Soni', 'Joel', 'Maz'].map((n, i) => (
								<ListRow
									key={n}
									leading={<Avatar name={n} />}
									title={n}
									trailing={
										<Menu
											trigger={<IconButton icon={Ellipsis} label="More" size="sm" />}
											items={menuItems}
											onSelect={setLastVerb}
											placement="bottom-end"
										/>
									}
								/>
							))}
						</List>
					</Panel>
				</Specimen>
			</Section>

			<Section
				id="toolbar"
				title="Toolbar"
				text="Floating rows of tools on the canvas: the tools themselves with the live one in red, the palette with the eyedropper, and a bar of labelled values."
			>
				<Specimen label="Tools" column>
					<div className="row">
						<Toolbar>
							<Tool
								icon={Plus}
								label="Add"
								onClick={() => setTool('add')}
								active={tool === 'add'}
							/>
							<ToolDivider />
							<Tool
								icon={MousePointer2}
								label="Select"
								onClick={() => setTool('select')}
								active={tool === 'select'}
							/>
							<ToolDivider />
							<Tool
								icon={Type}
								label="Text"
								onClick={() => setTool('text')}
								active={tool === 'text'}
							/>
							<ToolDivider />
							<Tool
								icon={LayoutGrid}
								label="Layout"
								onClick={() => setTool('layout')}
								active={tool === 'layout'}
							/>
							<ToolDivider />
							<Tool
								icon={Flower}
								label="Shapes"
								menu
								onClick={() => setTool('shapes')}
								active={tool === 'shapes'}
							/>
						</Toolbar>
					</div>
					<div className="row">
						<Swatches
							colors={palette}
							value={swatch}
							onChange={setSwatch}
							onPick={() => toast('Pick a colour')}
						/>
					</div>
					<div className="row">
						<Toolbar>
							<ToolField label="Color">
								<ToolValue leading={<Swatch color="var(--red-500)" label="Red" size="sm" />}>
									E34927
								</ToolValue>
							</ToolField>
							<ToolField label="Weight">
								<ToolValue leading={<Icon icon={PenLine} size="sm" />}>2</ToolValue>
							</ToolField>
							<ToolField label="End points">
								<Tool icon={MoveRight} label="Arrow end" />
								<Tool icon={Minus} label="Plain end" />
							</ToolField>
						</Toolbar>
					</div>
				</Specimen>
			</Section>

			<Section
				id="tile"
				title="Tile"
				text="A big coloured block to start from: an icon, a title, one verb, and room on the right for the art. Seven hues, each with its own ink."
			>
				<Specimen label="Hues" column>
					<div className="row-fields">
						<Tile hue="red" icon={Layers} title="Start with a template" />
						<Tile hue="navy" icon={LayoutGrid} title="Start with a surface" />
					</div>
					<div className="row-fields">
						<Tile hue="green" icon={Flower} title="Manifesto" action="Read" />
						<Tile hue="sky" icon={Sparkles} title="Starter" action="Choose" />
					</div>
					<div className="row-fields">
						<Tile hue="amber" icon={Star} title="Enterprise" action="Talk to us" />
						<Tile hue="purple" icon={Users} title="Made for visionaries" action="Take the leap" />
					</div>
				</Specimen>
			</Section>

			<Section
				id="jobs"
				title="Jobs"
				text="What the agent is doing right now. The head folds the list open and closed. Each job carries its verb in colour and what it is on; the queued ones fade below, the running mark is the house spinner, done is an amber check."
			>
				<Specimen label="Working">
					<Panel>
						<Jobs
							title={`Working on ${jobsNow.length - doneCount} jobs`}
							caption={doneCount ? `${doneCount} done` : 'Taking in your messages'}
							jobs={jobsNow}
							onOpen={() => toast('Opened the job')}
						/>
					</Panel>
				</Specimen>
			</Section>

			<Section
				id="invite"
				title="Invite"
				text="The light panel: who can get in, and how. One field with the act beside it, the people under it with their role, and the rows that lead somewhere."
			>
				<Specimen label="Invite to edit" column>
					<Panel tone="light">
						<h3 className="section-title">Invite people to edit</h3>
						<div className="row" style={{ flexWrap: 'nowrap' }}>
							<Input placeholder="Name or email to allow access" />
							<IconButton
								icon={Plus}
								label="Invite"
								variant="primary"
								onClick={() => toast('Invited')}
							/>
						</div>
						<span className="status-line">Who has access</span>
						<List>
							<ListRow
								leading={<Avatar name="Sarah Dalton" hue="teal" />}
								title="Sarah Dalton"
								subtitle="sarah@acme.com"
								trailing={
									<Select
										size="sm"
										value={role}
										onChange={setRole}
										options={[
											{ value: 'editor', label: 'Editor' },
											{ value: 'viewer', label: 'Viewer' },
											{ value: 'admin', label: 'Admin' }
										]}
									/>
								}
							/>
						</List>
					</Panel>
					<Panel tone="light">
						<List>
							<ListRow
								title="Open prototype link"
								subtitle="Contains code"
								trailing={
									<>
										<Button icon={Copy} onClick={() => toast('Link copied')}>
											Copy link
										</Button>
										<Button trailing={ChevronsUpDown}>Team access</Button>
									</>
								}
							/>
							<ListRow
								leading={
									<Icon
										icon={Flower}
										size="lg"
										className="icon icon-lg"
										style={{ color: 'var(--red-500)' }}
									/>
								}
								title="Publish prototype template"
								trailing={<Icon icon={ChevronRight} />}
								onClick={() => toast('Publish')}
							/>
						</List>
					</Panel>
				</Specimen>
				<Specimen label="Invite to access" column>
					<Panel tone="light">
						<h3 className="section-title">Invite people to access and edit</h3>
						<div className="row" style={{ flexWrap: 'nowrap' }}>
							<Input placeholder="Enter email" />
							<Button variant="primary" icon={Plus} onClick={() => toast('Invited')}>
								Invite
							</Button>
						</div>
						<List>
							{(
								[
									['Lucie Loubet', 'Editor and admin'],
									['Nim', 'Editor'],
									['Xchyler', 'Viewer']
								] as const
							).map(([n, r]) => (
								<ListRow key={n} leading={<Avatar name={n} />} title={n} subtitle={r} />
							))}
						</List>
						<Property label="Anyone with the link">
							<Segmented
								value={access}
								onChange={setAccess}
								tabs={[
									{ id: 'viewer', label: 'Viewer', icon: Eye },
									{ id: 'editor', label: 'Editor', icon: PenLine }
								]}
							/>
						</Property>
						<div className="row">
							<Button icon={Copy} onClick={() => toast('Link copied')}>
								Copy link
							</Button>
						</div>
					</Panel>
				</Specimen>
			</Section>

			<Section
				id="record"
				title="Record"
				text="One record as a page, on the same top a document has: a banner edge to edge, the emoji half over its lower edge, the name below it, the way back in the banner's corner, a property per row with its glyph and a quiet editor that reads as text until you are on it, the notes as prose, then the rows that point back at it. Every edit is its own write."
			>
				<Specimen label="Client" column plain>
					<Fold
						id="specimen-record"
						wide={720}
						compact={400}
						note="Drag the seam in. Under 520 the page pulls in, the name comes down a size, and a property’s name sits above its editor."
					>
						<Record
							cover="https://images.unsplash.com/photo-1606811841689-23dfddce3e95?w=1200&q=70&auto=format&fit=crop"
							emoji="🦷"
							hue="teal"
							title={client.name}
							onTitleChange={edit('name')}
							onClose={() => toast('Closed')}
						>
							<RecordProperties>
								<RecordProperty icon={MapPin} label="Town">
									<Input
										quiet
										value={client.town}
										onChange={(e) => edit('town')(e.target.value)}
										placeholder="Where it is"
									/>
								</RecordProperty>
								<RecordProperty icon={Globe} label="Website">
									<Input
										quiet
										value={client.website}
										onChange={(e) => edit('website')(e.target.value)}
										placeholder="Their site"
									/>
								</RecordProperty>
								<RecordProperty icon={Phone} label="Phone">
									<Input
										quiet
										value={client.phone}
										onChange={(e) => edit('phone')(e.target.value)}
										placeholder="The front desk"
									/>
								</RecordProperty>
								<RecordProperty icon={Briefcase} label="Vertical">
									<Select
										quiet
										value={client.vertical}
										onChange={edit('vertical')}
										options={[
											{ value: 'dental', label: 'Dental' },
											{ value: 'accounting', label: 'Accounting' },
											{ value: 'ecommerce', label: 'Ecommerce' }
										]}
									/>
								</RecordProperty>
								<RecordProperty icon={TrendingUp} label="Stage">
									<Select
										quiet
										value={client.stage}
										onChange={edit('stage')}
										options={[
											{ value: 'developing', label: 'Developing' },
											{ value: 'early', label: 'Early' },
											{ value: 'strong', label: 'Strong' },
											{ value: 'stalled', label: 'Stalled' },
											{ value: 'won', label: 'Won' },
											{ value: 'lost', label: 'Lost' }
										]}
									/>
								</RecordProperty>
								<RecordProperty icon={Banknote} label="Contract value">
									<Select
										quiet
										value={client.acv}
										onChange={edit('acv')}
										options={[
											{ value: 'under 10k', label: 'Under 10k' },
											{ value: '10k to 25k', label: '10k to 25k' },
											{ value: '25k to 50k', label: '25k to 50k' },
											{ value: 'over 50k', label: 'Over 50k' }
										]}
									/>
								</RecordProperty>
								<RecordProperty icon={Users} label="Owner">
									<span className="row" style={{ gap: 'var(--control-gap)' }}>
										<Avatar name="Marijn" size="sm" hue="green" />
										<span>Marijn</span>
									</span>
								</RecordProperty>
								<RecordProperty icon={CalendarDays} label="Last touch">
									<Input
										quiet
										value={client.lastTouch}
										onChange={(e) => edit('lastTouch')(e.target.value)}
									/>
								</RecordProperty>
								<RecordProperty icon={Flag} label="Next action">
									<Badge hue="amber">Claims desk demo on the 12th</Badge>
								</RecordProperty>
							</RecordProperties>
							<RecordDivider />
							<RecordBody
								value={client.notes}
								onChange={edit('notes')}
								placeholder="What has been learned"
							/>
							<RecordDivider />
							<RecordLinks title="Meetings">
								<RecordLink
									icon={Calendar}
									label="Claims desk walkthrough with Yolanda"
									detail="September 2"
									onClick={() => toast('Opened the meeting')}
								/>
								<RecordLink
									icon={Calendar}
									label="Kickoff with Ian"
									detail="August 19"
									onClick={() => toast('Opened the meeting')}
								/>
							</RecordLinks>
							<RecordLinks title="People">
								<RecordLink
									icon={Users}
									label="Yolanda Price"
									detail="Front desk"
									onClick={() => toast('Opened the person')}
								/>
								<RecordLink
									icon={Users}
									label="Ian Reyes"
									detail="Owner"
									onClick={() => toast('Opened the person')}
								/>
							</RecordLinks>
							<RecordLinks title="Tasks">
								<RecordLink
									icon={SquareCheck}
									label="A claim's note writes back to Sikka"
									detail="Done"
									onClick={() => toast('Opened the task')}
								/>
								<RecordLink
									icon={SquareCheck}
									label="The plan record shows the payer's real fee schedule"
									detail="Doing"
									onClick={() => toast('Opened the task')}
								/>
							</RecordLinks>
						</Record>
					</Fold>
				</Specimen>
			</Section>

			<Section
				id="board"
				title="Board"
				text="The pipeline as lanes by stage. A card lifts on a short drag and follows the pointer; the lane it crosses opens a slot for it, and the drop settles it. Arrow keys move a card too. Each move is one write of the stage."
			>
				<Specimen label="Deals" column plain>
					<Fold
						id="specimen-board"
						wide={960}
						compact={400}
						note="Drag the seam in. Under 720 the lanes narrow and a card is its title; under 480 the lanes stack into one list."
					>
						<Board
							lanes={stages}
							cards={deals}
							onChange={setDeals}
							onOpen={(d) => toast(`Opened ${d.name}`)}
							onAdd={(lane) => toast(`New deal in ${lane}`)}
							renderCard={(d) => (
								<CardBody
									title={d.name}
									subtitle={d.town}
									foot={
										<>
											<Avatar name={d.owner.name} size="sm" hue={d.owner.hue} />
											<Badge hue="gray">{d.acv}</Badge>
										</>
									}
								/>
							)}
						/>
					</Fold>
				</Specimen>
			</Section>

			<Section
				id="pipeline"
				title="Pipeline"
				text="Every client on one page and how far each has come. The stages run across the top, a client is a row, and its track fills to the stage it stands in: solid where it is now, soft behind it, empty ahead. Where we have been round a stage more than once, its rectangle is cut into one slice a pass, so a third round reads as three. Then how long we have been on the client, and everyone working on it. Press a stage on a row and the client moves there, which is one write of the stage. A pipeline given no move is read only, and holds nothing to tab through."
			>
				<Specimen label="Clients" column plain>
					<Fold
						id="specimen-pipeline"
						wide={960}
						compact={400}
						note="Drag the seam in. Under 560 the stage labels go and each client says in words where it stands; under 400 the row stacks and the track runs the full width."
					>
						<Pipeline
							clients={engagements}
							stages={fdeStages}
							today={new Date(2026, 8, 4)}
							onOpen={(c) => toast(`Opened ${c.name}`)}
							onMove={(c, stage) => {
								setEngagements((cs) => cs.map((x) => (x.id === c.id ? { ...x, stage } : x)));
								toast(`${c.name} moved to ${fdeStages.find((s) => s.id === stage)?.label}`);
							}}
						/>
					</Fold>
				</Specimen>
				<Specimen label="Passes" column plain>
					<Pipeline clients={passRows} stages={fdeStages} today={new Date(2026, 8, 4)} />
				</Specimen>
				<Specimen label="Time on it" column plain>
					<Pipeline clients={spanRows} stages={fdeStages} today={new Date(2026, 8, 4)} />
				</Specimen>
				<Specimen label="Everyone on it" column plain>
					<Pipeline clients={crewRows} stages={fdeStages} today={new Date(2026, 8, 4)} />
				</Specimen>
			</Section>

			<Section
				id="outreach"
				title="Outreach"
				text="The accounts we are going after, grouped by where they stand. Each with its score as a small meter and the number, who we know there as marks, and the vertical it is in. Press a score and the reasoning opens: the sentence, and fit, timing and connection under it."
			>
				<Specimen label="Target accounts" column plain>
					<TargetsBar>
						<div className="row">
							<Segmented
								value={targetTab}
								onChange={setTargetTab}
								tabs={[
									{ id: 'all', label: 'All' },
									{ id: 'Identified', label: 'Identified' },
									{ id: 'In motion', label: 'In motion' },
									{ id: 'Queued', label: 'Queued' }
								]}
							/>
							<Button variant="ghost" size="sm" icon={ListFilter}>
								Filter
							</Button>
							<Button variant="ghost" size="sm" icon={SlidersHorizontal}>
								Display
							</Button>
						</div>
						<div className="row">
							<Button size="sm" icon={Upload} onClick={() => toast('Import a CSV')}>
								Import
							</Button>
							<Button variant="primary" size="sm" icon={Plus} onClick={() => toast('New account')}>
								Add account
							</Button>
						</div>
					</TargetsBar>
					<Fold
						id="specimen-outreach"
						wide={880}
						compact={480}
						note="Drag the seam in. Under 560 the account and its score stand alone."
					>
						<Targets
							targets={shownTargets}
							stages={targetStages}
							onOpen={(t) => toast(`Opened ${t.name}`)}
						/>
					</Fold>
				</Specimen>
				<Specimen label="Score">
					<Score
						value={93}
						why="Clear fit at the right moment."
						parts={[
							{ label: 'Fit', value: 88 },
							{ label: 'Timing', value: 97 },
							{ label: 'Connection', value: 94 }
						]}
					/>
					<Score
						value={72}
						why="Right shape, a quarter early."
						parts={[
							{ label: 'Fit', value: 80 },
							{ label: 'Timing', value: 55 },
							{ label: 'Connection', value: 70 }
						]}
					/>
					<Score value={48} />
				</Specimen>
			</Section>

			<Section
				id="calendar"
				title="Calendar"
				text="The month as a grid of weeks, or one week as seven columns, the meetings on their days as Notion's calendar has its pages: an emoji and a title on a tint. A day's number sits at its top left, today's in the act colour, the first of a month with its month. A plus shows on the day under the pointer; press it, or the day, and a new meeting is named where it will sit. Press a meeting to open it."
			>
				<Specimen label="Meetings" column plain>
					<Fold
						id="specimen-calendar"
						wide={880}
						compact={380}
						note="Drag the seam in. Under 640 the times go; under 440 a day is its first letter and a month’s events are bars in their hue."
					>
						<MonthCalendar
							cursor={cursor}
							onCursorChange={setCursor}
							view={calView}
							onViewChange={setCalView}
							events={meetings}
							today={new Date(2026, 8, 4)}
							onCreate={(d, t) => toast(`${t} on ${d}`)}
							onOpen={(e) => toast(`Opened ${e.title}`)}
						/>
					</Fold>
				</Specimen>
			</Section>

			<Section
				id="call"
				title="Call"
				text="Our own call, for support and sales. Tiles fill a grid by how many are in it; a shared screen takes the stage and the people move to a strip beside it. The chat is a column of its own on a seam, a chat like any other. Whoever is speaking wears the act colour. Controls along the bottom, the chat as a pane on the right. Video, chat, screen: nothing else."
			>
				<Specimen label="In a call" column plain>
					<div className="call-stage-frame">
						<Split id="specimen-call" side="end" width={320} min={240} max={560}>
							<Call
								title="Payroll with Kaisey and Ian"
								elapsed="12:41"
								people={callPeople.map((p) =>
									p.me ? { ...p, muted: !mic, camera: cam, presenting: sharing } : p
								)}
								mic={mic}
								camera={cam}
								sharing={sharing}
								chatOpen={callChatOpen}
								onMic={() => setMic((m) => !m)}
								onCamera={() => setCam((c) => !c)}
								onShare={() => setSharing((s) => !s)}
								onChat={() => setCallChatOpen((o) => !o)}
								onLeave={() => toast('Left the call')}
							/>
							<div className="call-chat-column">
								<CallChat
									lines={callLines}
									footer={
										<Composer
											value={callLine}
											onChange={setCallLine}
											onSend={() => {
												setCallLines((l) => [
													...l,
													{
														id: String(Date.now()),
														from: 'Marijn',
														when: 'now',
														text: callLine.trim(),
														mine: true
													}
												]);
												setCallLine('');
											}}
											placeholder="Say something"
										/>
									}
								/>
							</div>
						</Split>
					</div>
					<div className="row">
						<Button
							onClick={() =>
								setCallPeople((p) =>
									p.length < 9
										? [
												...p,
												{
													id: String(p.length),
													name: guests[(p.length - 3) % guests.length],
													camera: p.length % 2 === 0
												}
											]
										: p
								)
							}
						>
							Someone joins
						</Button>
						<Button
							variant="ghost"
							onClick={() => setCallPeople((p) => (p.length > 1 ? p.slice(0, -1) : p))}
						>
							Someone leaves
						</Button>
						<Button
							variant="ghost"
							onClick={() =>
								setCallPeople((p) =>
									p.map((x, i) => ({
										...x,
										speaking: i === (p.findIndex((y) => y.speaking) + 1) % p.length
									}))
								)
							}
						>
							Next speaker
						</Button>
					</div>
				</Specimen>
			</Section>

			<Section
				id="timeline"
				title="Timeline"
				text="History as a story, not a list: one block per branch, newest first, with its name, a sentence of what it did, and when. The graph leaves the trunk where the branch started and comes back where it merged; an open branch ends at its node."
			>
				<Specimen label="Branches" column plain>
					<Timeline entries={story} onOpen={(e) => toast(`Opened ${e.name}`)} />
				</Specimen>
			</Section>

			<Section
				id="meter"
				title="Meter"
				text="A share of one as a row of bars. Amber while it runs and the head bar breathes, green when it is done, red when it failed."
			>
				<Specimen label="Tones" column>
					<div className="row">
						<Meter value={meterValue} tone="running" />
						<span className="status-line">{Math.round(meterValue * 100)}% running</span>
					</div>
					<div className="row">
						<Meter value={1} tone="done" />
						<span className="status-line">Done</span>
					</div>
					<div className="row">
						<Meter value={0.66} tone="failed" />
						<span className="status-line">Failed on the third step</span>
					</div>
					<div className="row">
						<Meter value={0.2} tone="idle" />
						<span className="status-line">Idle</span>
					</div>
					<div className="row">
						<Button onClick={() => setMeterValue((v) => Math.min(1, v + 0.1))}>Advance</Button>
						<Button variant="ghost" onClick={() => setMeterValue(0.1)}>
							Reset
						</Button>
					</div>
				</Specimen>
			</Section>

			<Section
				id="editor"
				title="Editor"
				text="A document, written. Tiptap underneath, headless, so the words wear the system's type and nothing else. Markdown goes in and comes out. The marks are a row above and a small menu on whatever you select; typing markdown works too, a dash for a list, a hash for a heading. No code marks, the house has no monospace."
			>
				<Specimen label="A note" column plain>
					<Editor value={doc} onChange={setDoc} placeholder="Write the note" />
				</Specimen>
				<Specimen label="Read only" column plain>
					<Editor value={doc} readOnly />
				</Specimen>
			</Section>

			<Section
				id="cover"
				title="Cover"
				text="The top of a page someone made: a banner edge to edge, the page's emoji half over its lower edge, and the name below it. Without a picture the banner is a tint. A document and a client record are the same thing seen twice, so both stand on this and neither can drift from the other. What differs is only the column under it, which the page supplies."
			>
				<Specimen label="With a picture" column plain>
					<div className="document-stage">
						<Cover
							cover="https://images.unsplash.com/photo-1497215728101-856f4ea42174?w=1200&q=70&auto=format&fit=crop"
							emoji="🏢"
							title="Whose page this is"
						>
							<p>Whatever the page is goes here, in the column under the name.</p>
						</Cover>
					</div>
				</Specimen>
				<Specimen label="Without one" column plain>
					<div className="document-stage">
						<Cover hue="amber" emoji="🧾" title="A page with no picture yet">
							<p>The banner is the page's own tint until someone gives it one.</p>
						</Cover>
					</div>
				</Specimen>
			</Section>

			<Section
				id="document"
				title="Document"
				text="A Cover with the editor under it: the banner, the emoji over its edge and the title, then the words in one centred column of a comfortable measure at the reading size. The emoji and the title change in place; the editor has no bar, its marks on whatever is selected."
			>
				<Specimen label="Page" column plain>
					<div className="document-stage">
						<Document emoji={docEmoji} onEmoji={setDocEmoji} title={docTitle} onTitle={setDocTitle}>
							<Editor value={docBody} onChange={setDocBody} toolbar={false} placeholder="Write" />
						</Document>
					</div>
				</Specimen>
			</Section>

			<Section
				id="avatar"
				title="Avatar"
				text="A person as their photo, or an animal from their name on a soft tint, so the same person is the same fox everywhere. The face fills its tint at every size. The animal and the tint follow the name unless given. Never initials."
			>
				<Specimen label="Sizes">
					<Avatar name="Kaisey Ahmed" size="sm" />
					<Avatar name="Kaisey Ahmed" />
					<Avatar name="Kaisey Ahmed" size="lg" />
					<Avatar name="Ian Reyes" size="lg" src="https://i.pravatar.cc/96?img=12" />
				</Specimen>
				<Specimen label="Faces">
					{[
						'Kaisey Ahmed',
						'Ian Reyes',
						'Yolanda Price',
						'Lulu Chen',
						'Marijn',
						'Lucie Loubet',
						'Nim',
						'Xchyler',
						'Angie Torres',
						'Rosa Martinez',
						'Devin Lo',
						'Marcus Bell',
						'Priya Nair',
						'Howard Kim',
						'Lena Okafor',
						'Tomas Reyes'
					].map((n) => (
						<Tooltip key={n} label={n}>
							<Avatar name={n} size="lg" />
						</Tooltip>
					))}
				</Specimen>
			</Section>

			<Section
				id="list"
				title="List"
				text="A title, a line under it, and something on the right. Hairlines between the rows, a tint on hover, nothing in front of the name."
			>
				<Specimen label="People" column plain>
					<List>
						{people.map((p) => (
							<ListRow
								key={p.id}
								title={p.name}
								subtitle={`${p.role} at ${p.client}`}
								trailing={
									<Badge
										hue={p.status === 'Active' ? 'green' : p.status === 'Away' ? 'amber' : 'gray'}
									>
										{p.status}
									</Badge>
								}
								onClick={() => toast(`Opened ${p.name}`)}
							/>
						))}
					</List>
				</Specimen>
			</Section>

			<Section id="table" title="Table" text="A quiet head, hairline rows, numbers on the right.">
				<Specimen label="Hours by person" column plain>
					<Table
						columns={columns}
						rows={people}
						rowKey={(p) => p.id}
						onRowClick={(p) => toast(`Opened ${p.name}`)}
					/>
				</Specimen>
			</Section>

			<Section
				id="sheet"
				title="Sheet"
				text="Lulu's grid, on the tokens. Every cell is an input: click and you are typing, drag to select a range, shift and the arrows extend it, copy is tab separated, a block from Excel pastes where you stand. The cell with the caret grows to its words. Both axes are virtualised and the first column is pinned."
			>
				<Specimen label="Pay sheet" column plain>
					<Sheet
						columns={sheetColumns}
						rows={sheetRows}
						pinned={['name']}
						onChange={(rowId, columnId, value) =>
							setSheetRows((rs) =>
								rs.map((r) =>
									r.id === rowId ? { ...r, values: { ...r.values, [columnId]: value } } : r
								)
							)
						}
						onFill={(changes) =>
							setSheetRows((rs) => {
								const byRow = new Map<string, SheetChange[]>();
								for (const c of changes) byRow.set(c.rowId, [...(byRow.get(c.rowId) ?? []), c]);
								return rs.map((r) => {
									const mine = byRow.get(r.id);
									if (!mine) return r;
									const values = { ...r.values };
									for (const c of mine) values[c.columnId] = c.value;
									return { ...r, values };
								});
							})
						}
						onAddRow={() =>
							setSheetRows((rs) => [
								...rs,
								{ id: String(Date.now()), values: { name: 'New employee' } }
							])
						}
					/>
				</Specimen>
			</Section>
		</div>
	);
}

export function App() {
	const main = useRef<HTMLElement | null>(null);
	const [active, setActive] = useScrollSpy(sectionIds, main);
	const [dark, setDark] = useTheme();
	const [workspace, setWorkspace] = useState('Mulholland');
	const [query, setQuery] = useState('');
	const visible = useMemo(() => {
		const q = query.trim().toLowerCase();
		if (!q) return sections;
		return sections
			.map((g) => ({ ...g, items: g.items.filter((i) => i.label.toLowerCase().includes(q)) }))
			.filter((g) => g.items.length);
	}, [query]);
	const go = useCallback(
		(id: string) => {
			setActive(id);
			document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
			history.replaceState(null, '', `#${id}`);
		},
		[setActive]
	);

	return (
		<ToastProvider>
			<div className="app">
				<div className="app-rail">
					<Sidebar
						sections={visible}
						active={active}
						onSelect={go}
						header={
							<SidebarHead
								workspace={
									<Menu
										trigger={<Workspace name={workspace} emoji="🧩" />}
										items={[
											{ heading: 'Workspaces' },
											{ id: 'Mulholland', label: 'Mulholland' },
											{ id: 'Marzy', label: 'Marzy' }
										]}
										onSelect={setWorkspace}
									/>
								}
							/>
						}
						footer={
							<>
								<Input
									size="sm"
									icon={Search}
									placeholder="Search"
									value={query}
									onChange={(e) => setQuery(e.target.value)}
								/>
								<Tooltip label={dark ? 'Light' : 'Dark'} placement="top">
									<IconButton
										icon={dark ? Sun : Moon}
										label={dark ? 'Light' : 'Dark'}
										onClick={() => setDark(!dark)}
									/>
								</Tooltip>
							</>
						}
					/>
				</div>
				<main ref={main} className="app-main">
					<Page />
				</main>
			</div>
		</ToastProvider>
	);
}
