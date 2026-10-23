// All page copy and lists live here so the page markup stays small.

export const repo = 'https://github.com/Mulholland-Inc/archive';
export const image = 'ghcr.io/mulholland-inc/archive';
export const email = 'hello@archive.us.org';

export const sources = [
	'Gmail',
	'Google Drive',
	'Google Photos',
	'iCloud',
	'Dropbox',
	'OneDrive',
	'Outlook',
	'Notion',
	'Slack',
	'GitHub',
	'Stripe',
	'Shopify',
	'QuickBooks',
	'Plaid',
	'Spotify',
	'Strava',
	'Calendar',
	'Contacts',
	'WhatsApp export',
	'Instagram export',
	'Any IMAP inbox',
	'A folder of PDFs',
	'CSV & Excel',
	'Postgres & MySQL'
];

export const assistants = ['Claude', 'ChatGPT', 'Gemini', 'Copilot', 'Cursor', 'Anything with MCP'];

export const steps = [
	{
		title: 'Plug it in',
		body: 'Connect your inbox, your photos, your bank, or a folder of PDFs. Archive copies all of it into one box on your laptop or your own cloud.'
	},
	{
		title: 'It tidies up',
		body: 'A local AI reads each thing and files it as a proper record: a receipt, a flight, a contract, a person. Every field says how sure it is, and when it’s unsure, it asks you.'
	},
	{
		title: 'Ask it anything',
		body: 'Browse it like Notion, query it from the terminal, or let Claude and ChatGPT look things up for you. They only see what you allow.'
	}
];

export const features = [
	{
		tag: 'one box',
		title: 'Runs anywhere',
		body: 'One Docker container with Postgres inside. Your laptop, a Raspberry Pi under the desk, or any cloud you like.'
	},
	{
		tag: 'types',
		title: 'Real types, not text',
		body: 'Money knows its currency. Dates know their time zone. An SSN knows it’s secret and stays encrypted. About 40 types come built in.'
	},
	{
		tag: 'files',
		title: 'Files are first-class',
		body: 'PDFs, photos, scans and screenshots get previews, OCR and full-text search, plus a gallery that’s actually nice to scroll.'
	},
	{
		tag: 'mcp',
		title: 'Memory for your AI',
		body: 'One MCP server gives Claude, ChatGPT and friends a long memory of your life, with read and write permissions you set per app.'
	},
	{
		tag: 'yaml',
		title: 'Shape it your way',
		body: 'Your schema is plain YAML. Edit it yourself or let an agent propose a change, preview it on your real data, then apply or undo.'
	},
	{
		tag: 'exit',
		title: 'Leave whenever',
		body: 'It’s Postgres and plain files underneath. Export everything at any time in formats any tool can read. No lock-in, ever.'
	}
];

export const plans = [
	{
		name: 'Self-hosted',
		price: '$0',
		per: 'forever',
		blurb: 'The whole thing, open source. Run it on your own hardware.',
		points: ['Every feature, no limits', 'One Docker container', 'Bring your own local model', 'Community support'],
		cta: 'Install guide',
		href: repo,
		featured: false
	},
	{
		name: 'Archive Cloud',
		price: '$8',
		per: 'per month',
		blurb: 'We run it for you at archive.us.org. Same open source, zero chores.',
		points: ['Automatic updates and backups', 'Your own encryption key', 'Connect your AI apps in one click', 'Export or move to self-hosted anytime'],
		cta: 'Join the waitlist',
		href: '#waitlist',
		featured: true
	},
	{
		name: 'Business',
		price: 'Let’s talk',
		per: '',
		blurb: 'A clean layer over the systems you already run. No migration.',
		points: ['Installs on your cloud', 'Custom connectors and types', 'Support with real SLAs', 'Help from the people who built it'],
		cta: 'Get in touch',
		href: `mailto:${email}`,
		featured: false
	}
];

export const faqs = [
	{
		q: 'Is it really free?',
		a: 'Yes. Archive is open source and the self-hosted version has every feature. We make money by hosting it for people who’d rather not, and by helping businesses set it up.'
	},
	{
		q: 'Where does my data live?',
		a: 'Wherever you run it. Self-hosted, it never leaves your machine. On Archive Cloud, it’s stored encrypted with a key you hold, and you can download all of it whenever you want.'
	},
	{
		q: 'Does an AI read my stuff?',
		a: 'Only one you choose. By default the tidying-up runs on a local model on your own computer. Cloud models and AI apps only get in if you connect them, and only to the parts you allow.'
	},
	{
		q: 'How sure is “sure”?',
		a: 'Every field Archive fills in carries a confidence score, and the original is always kept next to it. Anything below your threshold waits in a review pile instead of being guessed.'
	},
	{
		q: 'Can I leave?',
		a: 'Any time. Underneath it’s just Postgres and plain files, so a full export is one command and works with every tool you already have.'
	},
	{
		q: 'Is it for people or companies?',
		a: 'Both. People use it to keep their digital life in one place. Companies put it on top of their CRM, drives and inboxes so their team and their agents can ask one clean source.'
	}
];
