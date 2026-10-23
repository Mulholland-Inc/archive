<!-- Hero demo: a messy forwarded receipt becomes a typed record with confidence. -->
<script lang="ts">
	const fields = [
		{ key: 'merchant', value: 'Blue Bottle Coffee', type: 'text', sure: 99 },
		{ key: 'total', value: '$14.50 USD', type: 'money', sure: 99 },
		{ key: 'when', value: 'Sat 18 Oct 2026, 9:42 am PDT', type: 'time', sure: 97 },
		{ key: 'paid with', value: 'Visa •••• 4242', type: 'card', sure: 100, secret: true },
		{ key: 'category', value: 'Coffee', type: 'tag', sure: 71 }
	];
</script>

<div class="relative mx-auto w-full max-w-[30rem]">
	<!-- the mess -->
	<div
		class="relative z-0 w-[82%] -rotate-3 rounded-md border border-line bg-card p-4 font-mono text-[0.68rem] leading-relaxed text-dust shadow-[0_8px_24px_-12px_rgb(23_22_26/0.35)]"
	>
		<p class="text-ink">Fwd: Fwd: your receipt 🧾</p>
		<p class="mt-2">---------- Forwarded message ---------</p>
		<p>BLUE BOTTLE #0231 &nbsp; HAYES VALLEY</p>
		<p>1x OAT CORTADO ......... 6.25</p>
		<p>1x LIEGE WAFFLE ........ 7.00</p>
		<p>TAX ..................... 1.25</p>
		<p class="text-ink">TOTAL &nbsp; 14.50</p>
		<p>VISA XXXX4242 &nbsp; 10/18/26 09:42</p>
	</div>

	<!-- the record -->
	<div
		class="relative z-10 -mt-5 ml-auto w-[92%] rotate-1 rounded-xl border border-line bg-card shadow-[0_24px_48px_-20px_rgb(23_22_26/0.45)]"
	>
		<div class="flex items-center justify-between gap-3 border-b border-dashed border-line px-5 py-3">
			<span class="tape">receipt</span>
			<span class="tape blue animate-[wiggle_3s_ease-in-out_infinite]">filed ✓</span>
		</div>
		<dl class="divide-y divide-line/70 px-5 py-1">
			{#each fields as f, i (f.key)}
				<div
					class="grid grid-cols-[4.5rem_1fr_auto] sm:grid-cols-[5.5rem_1fr_auto] items-baseline gap-3 py-2.5 animate-[snap_.5s_ease-out_both]"
					style="animation-delay: {300 + i * 120}ms"
				>
					<dt class="text-sm text-dust">{f.key}</dt>
					<dd class="min-w-0 font-medium leading-snug">
						{f.value}
						{#if f.secret}<span class="ml-1 align-middle text-xs" title="Encrypted">🔒</span>{/if}
					</dd>
					<dd class="flex items-center gap-2 font-mono text-[0.65rem] tabular-nums">
						<span class="rounded bg-blue-soft px-1.5 py-0.5 text-blue">{f.type}</span>
						<span class={f.sure < 80 ? 'text-dust' : 'text-ink'}>{f.sure}%</span>
					</dd>
				</div>
			{/each}
		</dl>
		<p class="flex items-center gap-2 border-t border-dashed border-line px-5 py-3 text-sm text-dust">
			<span class="inline-block size-2 shrink-0 rounded-full bg-blue"></span>
			<span>Category is a guess, so it’s waiting for you in <b class="font-semibold text-ink">Review</b>.</span>
		</p>
	</div>
</div>
