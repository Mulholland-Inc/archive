<script lang="ts">
	import Chat from '#lib/Chat.svelte';
	import { email, faqs, image, plans, promises, repo, sources, steps } from '#lib/content';

	const pill =
		'inline-flex items-center justify-center rounded-full bg-ink px-7 py-3.5 text-[1.0625rem] font-medium text-paper transition-transform hover:-translate-y-0.5';
	const link = 'underline decoration-hair decoration-2 underline-offset-[6px] transition-colors hover:decoration-ink';
</script>

<svelte:head>
	<title>Archive · Your whole life, kept</title>
	<meta
		name="description"
		content="Archive gathers your email, photos, files and accounts into one private place you own. Then you can just ask. Open source and free to self-host."
	/>
	<meta property="og:title" content="Archive" />
	<meta property="og:description" content="Your whole life, kept." />
	<meta property="og:url" content="https://archive.us.org" />
	<meta name="theme-color" content="#fdfbf7" />
</svelte:head>

<header class="gutter">
	<nav class="mx-auto flex max-w-6xl items-center justify-between gap-6 py-7">
		<a href="/" class="font-serif text-[2rem] leading-none tracking-tight">archive</a>
		<div class="flex items-center gap-8 text-[1.0625rem]">
			<a class="hidden sm:inline hover:text-mute" href="#pricing">Pricing</a>
			<a class="hidden sm:inline hover:text-mute" href={repo}>GitHub</a>
			<a class="{pill} px-5 py-2.5 text-base" href="#waitlist">Get early access</a>
		</div>
	</nav>
</header>

<main>
	<!-- Hero -->
	<section class="gutter pt-20 pb-40 md:pt-32">
		<div class="mx-auto flex max-w-5xl flex-col items-center gap-10 text-center">
			<h1 class="text-[clamp(4rem,11vw,9.5rem)]">Your whole life, <em>kept.</em></h1>
			<p class="max-w-[36rem] text-[1.375rem] text-mute">
				Archive gathers your email, photos, files and accounts into one private place you own. Then you
				can just ask.
			</p>
			<div class="flex flex-wrap items-center justify-center gap-x-8 gap-y-4">
				<a class={pill} href="#waitlist">Get early access</a>
				<a class={link} href={repo}>Self-host it for free</a>
			</div>
		</div>
		<div class="mt-28">
			<Chat />
		</div>
	</section>

	<!-- Sources -->
	<section class="gutter pb-48">
		<div class="mx-auto max-w-5xl">
			<p class="font-serif text-[clamp(2.25rem,5.2vw,4.5rem)] leading-[1.08] tracking-[-0.015em] text-balance">
				It already knows where to look:
				{#each sources as s, i}<span class="text-mute">{s}</span>{i < sources.length - 1 ? ', ' : ''}{/each}<span
					class="text-mute">,</span
				>
				and that folder of PDFs on your desktop. <em>And</em> it talks to Claude, ChatGPT and Gemini.
			</p>
		</div>
	</section>

	<!-- How it works -->
	<section class="gutter pb-48">
		<div class="mx-auto grid max-w-6xl gap-16 md:grid-cols-3 md:gap-12">
			{#each steps as s, i}
				<div class="flex flex-col gap-4">
					<p class="font-serif text-[1.75rem] text-mute italic">{i + 1}.</p>
					<h2 class="text-[clamp(2.75rem,4.5vw,3.75rem)]">{s.title}</h2>
					<p class="max-w-[24rem] text-mute">{s.body}</p>
				</div>
			{/each}
		</div>
	</section>

	<!-- Promises -->
	<section class="gutter pb-48">
		<div class="mx-auto flex max-w-6xl flex-col gap-24">
			<h2 class="max-w-4xl text-[clamp(3.25rem,8vw,7rem)]">Built to be <em>kept</em> for a very long time.</h2>
			<div class="grid gap-x-20 gap-y-16 md:grid-cols-2">
				{#each promises as p}
					<div class="flex flex-col gap-3">
						<h3 class="text-[2.5rem]">{p.title}</h3>
						<p class="max-w-[30rem] text-mute">{p.body}</p>
					</div>
				{/each}
			</div>
			<code class="block max-w-full overflow-x-auto font-mono text-[0.9375rem] whitespace-nowrap text-mute"
				>$&nbsp;docker run -p 8080:8080 -v archive:/data {image}</code
			>
		</div>
	</section>

	<!-- For teams -->
	<section class="gutter pb-48">
		<div class="mx-auto flex max-w-4xl flex-col items-center gap-8 text-center">
			<h2 class="text-[clamp(3rem,7vw,6rem)]">Run a company? <em>Keep</em> your systems.</h2>
			<p class="max-w-[38rem] text-[1.375rem] text-mute">
				Archive sits on top of your CRM, books, drives and inboxes as one clean layer. Your team and your
				agents ask one place, and nothing has to move.
			</p>
			<a class={link} href="mailto:{email}">Talk to us</a>
		</div>
	</section>

	<!-- Pricing -->
	<section id="pricing" class="gutter scroll-mt-10 pb-48">
		<div class="mx-auto flex max-w-6xl flex-col gap-20">
			<h2 class="text-center text-[clamp(3rem,7vw,6rem)]">Free, or we run it <em>for you.</em></h2>
			<div class="grid gap-16 text-center md:grid-cols-3 md:gap-10">
				{#each plans as p}
					<div class="flex flex-col items-center gap-4">
						<p class="text-mute">{p.name}</p>
						<p class="font-serif text-[4.5rem] leading-none tracking-tight">{p.price}</p>
						<p class="max-w-[18rem] text-mute">{p.blurb}</p>
						<a class="{link} mt-2" href={p.href}>{p.cta}</a>
					</div>
				{/each}
			</div>
		</div>
	</section>

	<!-- FAQ -->
	<section class="gutter pb-48">
		<div class="mx-auto flex max-w-3xl flex-col gap-12">
			<h2 class="text-[clamp(3rem,7vw,6rem)]">Questions</h2>
			<div class="flex flex-col gap-2">
				{#each faqs as f}
					<details class="group py-3">
						<summary
							class="flex cursor-pointer list-none items-baseline justify-between gap-6 font-serif text-[2rem] leading-tight [&::-webkit-details-marker]:hidden"
						>
							{f.q}
							<span class="text-mute transition-transform group-open:rotate-45">+</span>
						</summary>
						<p class="max-w-[38rem] pt-3 text-mute">{f.a}</p>
					</details>
				{/each}
			</div>
		</div>
	</section>

	<!-- Waitlist -->
	<section id="waitlist" class="gutter scroll-mt-10 pb-40">
		<div class="mx-auto flex max-w-4xl flex-col items-center gap-10 text-center">
			<h2 class="text-[clamp(4rem,11vw,9.5rem)]">Start <em>keeping.</em></h2>
			<p class="max-w-[30rem] text-[1.375rem] text-mute">
				Archive Cloud opens to a small group first. Leave your email and we’ll send you an invite.
			</p>
			<!-- TODO: wire to the waitlist endpoint once app.archive.us.org exists. -->
			<form class="flex w-full max-w-md flex-col items-center gap-4 sm:flex-row" method="post" action="/waitlist">
				<label for="email" class="sr-only">Email</label>
				<input
					id="email"
					name="email"
					type="email"
					required
					placeholder="you@example.com"
					class="w-full min-w-0 flex-1 border-b-2 border-ink bg-transparent py-3 text-center text-[1.25rem] placeholder:text-mute focus:outline-none sm:text-left"
				/>
				<button type="submit" class="{pill} cursor-pointer">Save my spot</button>
			</form>
		</div>
	</section>
</main>

<footer class="gutter pb-10">
	<div class="mx-auto flex max-w-6xl flex-col gap-6 border-t border-hair pt-8 text-base text-mute sm:flex-row sm:items-center sm:justify-between">
		<p><span class="font-serif text-2xl text-ink">archive</span> &nbsp;No cookies, no trackers, no JavaScript on this page.</p>
		<div class="flex flex-wrap gap-6">
			<a class="hover:text-ink" href={repo}>GitHub</a>
			<a class="hover:text-ink" href="mailto:{email}">{email}</a>
			<span>© 2026 Mulholland</span>
		</div>
	</div>
</footer>
