<script lang="ts">
	import Intake from '#lib/Intake.svelte';
	import { assistants, email, faqs, features, image, plans, repo, sources, steps } from '#lib/content';

	const install = `docker run -p 8080:8080 -v archive:/data ${image}`;
	const btn =
		'inline-flex items-center justify-center gap-2 rounded-full px-6 py-3 font-semibold transition-transform hover:-translate-y-0.5 active:translate-y-0';
</script>

<svelte:head>
	<title>Archive · All your stuff, in one box you own</title>
	<meta
		name="description"
		content="Archive pulls your email, photos, files and accounts into one tidy, private database you own. Open source, self-hosted in one container, or hosted by us."
	/>
	<meta property="og:title" content="Archive" />
	<meta property="og:description" content="All your stuff, in one box you own." />
	<meta property="og:url" content="https://archive.us.org" />
	<meta name="theme-color" content="#f5f1e8" />
</svelte:head>

<!-- Nav -->
<header class="gutter sticky top-0 z-50 bg-paper/85 backdrop-blur-md">
	<nav class="mx-auto flex h-16 max-w-6xl items-center justify-between gap-6">
		<a href="/" class="flex items-center gap-2 font-display text-2xl font-extrabold tracking-tight">
			<img src="/favicon.svg" alt="" class="size-7" />
			archive
		</a>
		<div class="hidden items-center gap-7 text-[0.95rem] font-medium md:flex">
			<a class="hover:text-blue" href="#how">How it works</a>
			<a class="hover:text-blue" href="#connects">Connects to</a>
			<a class="hover:text-blue" href="#pricing">Pricing</a>
			<a class="hover:text-blue" href={repo}>GitHub</a>
		</div>
		<a href="#waitlist" class="{btn} bg-ink px-5 py-2.5 text-sm text-paper">Get early access</a>
	</nav>
</header>

<main>
	<!-- Hero -->
	<section class="gutter overflow-x-clip pt-10 pb-20 md:pt-16 md:pb-28">
		<div class="mx-auto grid max-w-6xl items-center gap-14 md:grid-cols-[1.1fr_1fr]">
			<div class="flex min-w-0 flex-col items-start gap-7">
				<span class="tape">open source · self-hosted · yours</span>
				<h1 class="text-[clamp(3.2rem,8vw,5.75rem)] font-extrabold">
					All your stuff, in one box <span class="text-blue">you&nbsp;own.</span>
				</h1>
				<p class="max-w-[34rem] text-xl text-dust">
					Archive pulls in your email, photos, files and accounts, then tidies them into one private
					database you can search, browse, and hand to your AI. Run it yourself for free, or let us
					run it for you.
				</p>
				<div class="flex flex-wrap gap-3">
					<a href="#waitlist" class="{btn} bg-blue text-white shadow-[0_6px_0_0_#17161a]">Join the waitlist</a>
					<a href={repo} class="{btn} border-2 border-ink">Star on GitHub</a>
				</div>
				<code
					class="block max-w-full overflow-x-auto rounded-lg bg-ink px-4 py-3 font-mono text-[0.72rem] whitespace-nowrap text-paper"
					><span class="select-none text-blue-soft/60">$&nbsp;</span>{install}</code
				>
			</div>
			<div class="min-w-0">
				<Intake />
			</div>
		</div>
	</section>

	<!-- Connects to -->
	<section id="connects" class="gutter scroll-mt-20 border-y border-line bg-card py-16">
		<div class="mx-auto flex max-w-6xl flex-col gap-8">
			<div class="flex flex-wrap items-end justify-between gap-4">
				<h2 class="text-4xl font-bold md:text-5xl">Plugs into the stuff you already use.</h2>
				<span class="tape">connectors in the works</span>
			</div>
			<ul class="flex flex-wrap gap-2.5">
				{#each sources as s}
					<li
						class="rounded-full border border-line bg-paper px-4 py-2 font-display text-lg font-semibold tracking-tight"
					>
						{s}
					</li>
				{/each}
			</ul>
			<div class="flex flex-wrap items-center gap-x-6 gap-y-3 border-t border-dashed border-line pt-6">
				<span class="font-mono text-xs tracking-[0.14em] text-dust uppercase">and talks to</span>
				{#each assistants as a}
					<span class="font-display text-xl font-bold text-blue">{a}</span>
				{/each}
			</div>
		</div>
	</section>

	<!-- How it works -->
	<section id="how" class="gutter scroll-mt-20 py-24">
		<div class="mx-auto flex max-w-6xl flex-col gap-12">
			<h2 class="max-w-3xl text-5xl font-extrabold md:text-6xl">
				Messy in. <span class="text-blue">Tidy</span> out.
			</h2>
			<ol class="grid gap-5 md:grid-cols-3">
				{#each steps as s, i}
					<li class="flex flex-col gap-4 rounded-2xl border border-line bg-card p-7">
						<span
							class="grid size-12 place-items-center rounded-full bg-blue font-display text-2xl font-extrabold text-white"
							>{i + 1}</span
						>
						<h3 class="text-3xl font-bold">{s.title}</h3>
						<p class="text-dust">{s.body}</p>
					</li>
				{/each}
			</ol>
		</div>
	</section>

	<!-- Features -->
	<section class="gutter pb-24">
		<div class="mx-auto flex max-w-6xl flex-col gap-12">
			<h2 class="max-w-3xl text-5xl font-extrabold md:text-6xl">
				Built like a filing cabinet. Feels like a&nbsp;toy.
			</h2>
			<div class="grid gap-x-10 gap-y-12 sm:grid-cols-2 lg:grid-cols-3">
				{#each features as f}
					<article class="flex flex-col items-start gap-3 border-t-2 border-ink pt-5">
						<span class="tape">{f.tag}</span>
						<h3 class="text-2xl font-bold">{f.title}</h3>
						<p class="text-dust">{f.body}</p>
					</article>
				{/each}
			</div>
		</div>
	</section>

	<!-- Business band -->
	<section class="gutter bg-blue py-20 text-white">
		<div class="mx-auto grid max-w-6xl items-center gap-10 md:grid-cols-[1.4fr_1fr]">
			<div class="flex flex-col items-start gap-5">
				<span class="tape">for teams</span>
				<h2 class="text-5xl font-extrabold md:text-6xl">Already have systems? Keep&nbsp;them.</h2>
				<p class="max-w-xl text-lg text-white/80">
					Archive sits on top of your CRM, ERP, shared drives and inboxes as one clean layer. Your team
					and your agents ask one place, with one set of definitions, and nothing has to move.
				</p>
			</div>
			<div class="rounded-2xl bg-white/10 p-6 font-mono text-sm leading-7 ring-1 ring-white/20">
				<p class="text-white/60"># ask across every system at once</p>
				<p>$ archive ask "unpaid invoices from</p>
				<p class="pl-4">customers we emailed this week"</p>
				<p class="mt-2 text-white/60">→ 7 records · HubSpot, Xero, Gmail</p>
			</div>
		</div>
	</section>

	<!-- Pricing -->
	<section id="pricing" class="gutter scroll-mt-20 py-24">
		<div class="mx-auto flex max-w-6xl flex-col gap-12">
			<div class="flex flex-col gap-3">
				<h2 class="text-5xl font-extrabold md:text-6xl">Simple pricing.</h2>
				<p class="text-xl text-dust">Same software everywhere. You just pick who runs it.</p>
			</div>
			<div class="grid items-start gap-5 md:grid-cols-3">
				{#each plans as p}
					<div
						class="flex flex-col gap-6 rounded-2xl p-7 {p.featured
							? 'bg-ink text-paper shadow-[0_10px_0_0_var(--color-blue)]'
							: 'border border-line bg-card'}"
					>
						<div class="flex items-center justify-between gap-3">
							<h3 class="text-2xl font-bold">{p.name}</h3>
							{#if p.featured}<span class="tape blue">most people</span>{/if}
						</div>
						<p class="flex items-baseline gap-2">
							<span class="font-display text-5xl font-extrabold tracking-tight">{p.price}</span>
							<span class={p.featured ? 'text-paper/60' : 'text-dust'}>{p.per}</span>
						</p>
						<p class={p.featured ? 'text-paper/75' : 'text-dust'}>{p.blurb}</p>
						<ul class="flex flex-col gap-2.5">
							{#each p.points as pt}
								<li class="flex gap-3">
									<span class="mt-[0.55em] size-2 shrink-0 rounded-full bg-blue"></span>{pt}
								</li>
							{/each}
						</ul>
						<a
							href={p.href}
							class="{btn} {p.featured ? 'bg-blue text-white' : 'border-2 border-ink'}"
							>{p.cta}</a
						>
					</div>
				{/each}
			</div>
		</div>
	</section>

	<!-- FAQ -->
	<section class="gutter pb-24">
		<div class="mx-auto grid max-w-6xl gap-10 md:grid-cols-[1fr_1.6fr]">
			<h2 class="text-5xl font-extrabold md:text-6xl">Good questions.</h2>
			<div class="flex flex-col border-t-2 border-ink">
				{#each faqs as f}
					<details class="group border-b border-line py-5">
						<summary
							class="flex cursor-pointer list-none items-center justify-between gap-6 font-display text-2xl font-bold tracking-tight [&::-webkit-details-marker]:hidden"
						>
							{f.q}
							<span
								class="grid size-8 shrink-0 place-items-center rounded-full border-2 border-ink text-lg transition-transform group-open:rotate-45"
								>+</span
							>
						</summary>
						<p class="max-w-[60ch] pt-3 text-dust">{f.a}</p>
					</details>
				{/each}
			</div>
		</div>
	</section>

	<!-- Waitlist -->
	<section id="waitlist" class="gutter scroll-mt-20 pb-24">
		<div
			class="mx-auto flex max-w-6xl flex-col items-start gap-6 rounded-[2rem] bg-ink px-8 py-14 text-paper md:px-14 md:py-20"
		>
			<span class="tape blue">early access</span>
			<h2 class="max-w-3xl text-5xl font-extrabold md:text-7xl">Get a box with your name on it.</h2>
			<p class="max-w-xl text-lg text-paper/70">
				Archive Cloud opens to a small group first. Leave your email and we’ll send you an invite. No
				spam, and you can probably guess what we think about selling your data.
			</p>
			<!-- TODO: wire to the waitlist endpoint once app.archive.us.org exists. -->
			<form class="flex w-full max-w-lg flex-col gap-3 sm:flex-row" method="post" action="/waitlist">
				<label for="email" class="sr-only">Email</label>
				<input
					id="email"
					name="email"
					type="email"
					required
					placeholder="you@example.com"
					class="min-w-0 flex-1 rounded-full bg-paper px-5 py-3 text-ink placeholder:text-dust focus:outline-none focus-visible:ring-4 focus-visible:ring-blue"
				/>
				<button type="submit" class="{btn} cursor-pointer bg-blue text-white">Save my spot</button>
			</form>
		</div>
	</section>
</main>

<footer class="gutter border-t border-line pt-14 pb-10">
	<div class="mx-auto flex max-w-6xl flex-col gap-10">
		<p class="font-display text-[clamp(4rem,18vw,13rem)] leading-[0.8] font-extrabold tracking-[-0.05em]">
			archive<span class="text-blue">.</span>
		</p>
		<div class="flex flex-wrap items-center justify-between gap-6 text-sm text-dust">
			<p>This page uses no cookies, no trackers, and no JavaScript. Obviously.</p>
			<div class="flex flex-wrap gap-6">
				<a class="hover:text-ink" href={repo}>GitHub</a>
				<a class="hover:text-ink" href="https://docs.archive.us.org">Docs</a>
				<a class="hover:text-ink" href="https://status.archive.us.org">Status</a>
				<a class="hover:text-ink" href="mailto:{email}">{email}</a>
			</div>
		</div>
		<p class="font-mono text-xs text-dust">© 2026 Mulholland · Made for keeping.</p>
	</div>
</footer>
