// All page copy and lists live here so the page markup stays small.

export const repo = 'https://github.com/Mulholland-Inc/archive';
export const image = 'ghcr.io/mulholland-inc/archive';
export const email = 'hello@archive.us.org';

export const sources = [
	'Gmail',
	'Google Photos',
	'iCloud',
	'Dropbox',
	'Notion',
	'Slack',
	'WhatsApp',
	'Spotify',
	'Strava',
	'Stripe',
	'your bank'
];

export const steps = [
	{
		title: 'Plug it in',
		body: 'Connect your inbox, your photos, your bank or a folder of PDFs. Archive copies all of it into one place you control.'
	},
	{
		title: 'It tidies up',
		body: 'A small AI on your own machine reads each thing and files it properly: a receipt, a flight, a lease, a person.'
	},
	{
		title: 'Just ask',
		body: 'Search it, browse it, or ask Claude or ChatGPT about it. They only see the parts you allow.'
	}
];

export const promises = [
	{
		title: 'Yours to keep',
		body: 'It runs in one container on your laptop, a home server or any cloud. Underneath it’s Postgres and plain files, so you can leave whenever you like.'
	},
	{
		title: 'Private by default',
		body: 'The sorting happens on your own machine. Card numbers, passport numbers and anything else sensitive stay encrypted.'
	},
	{
		title: 'Sure, or it asks',
		body: 'Every detail it files says how sure it is. If something looks shaky, it waits for you instead of guessing.'
	},
	{
		title: 'Photos and PDFs too',
		body: 'Scans, screenshots, receipts and albums get read, searched and kept in a gallery you’ll actually enjoy scrolling.'
	}
];

export const plans = [
	{
		name: 'Self-hosted',
		price: 'Free',
		blurb: 'Everything, open source, on your own hardware.',
		cta: 'Read the guide',
		href: repo
	},
	{
		name: 'Archive Cloud',
		price: '$8/mo',
		blurb: 'We run it for you, with backups and your own encryption key.',
		cta: 'Join the waitlist',
		href: '#waitlist'
	},
	{
		name: 'Business',
		price: 'Let’s talk',
		blurb: 'One clean layer over the systems your team already uses.',
		cta: 'Get in touch',
		href: `mailto:${email}`
	}
];

export const faqs = [
	{
		q: 'Is it really free?',
		a: 'Yes. Archive is open source and the self-hosted version has every feature. We make money by hosting it for people who’d rather not, and by helping companies set it up.'
	},
	{
		q: 'Does an AI read my stuff?',
		a: 'Only one you choose. By default the sorting runs on a model on your own computer. Other AI apps only get in if you connect them, and only to what you allow.'
	},
	{
		q: 'Can I leave?',
		a: 'Any time. It’s Postgres and plain files underneath, so a full export is one command and opens in tools you already have.'
	},
	{
		q: 'Is it for people or companies?',
		a: 'Both. People use it to keep their digital life in one place. Companies put it on top of their CRM, drives and inboxes so everyone, agents included, asks one clean source.'
	}
];
