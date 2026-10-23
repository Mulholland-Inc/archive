<!-- Hero demo: asking your archive a question, in a messages-style thread. -->
<script lang="ts">
	const thread = [
		{ me: true, text: 'when does my passport expire?' },
		{
			me: false,
			text: 'June 3, 2027. That’s about seven months from now. Want a reminder in January so you have time to renew?',
			source: 'From passport-receipt.pdf in Gmail'
		},
		{ me: true, text: 'yes pls. also how much did we spend on coffee this year 😬' },
		{ me: false, text: '$612.40 across 94 receipts. Blue Bottle is winning at $288.' }
	];
</script>

<div class="mx-auto flex w-full max-w-[34rem] flex-col gap-3 text-[1.0625rem] leading-snug">
	{#each thread as m, i}
		<div
			class="flex flex-col gap-1.5 {m.me ? 'items-end' : 'items-start'} animate-[rise_.6s_ease-out_both]"
			style="animation-delay: {250 + i * 450}ms"
		>
			<p
				class="max-w-[82%] rounded-[1.25rem] px-4 py-2.5 {m.me
					? 'rounded-br-md bg-blue text-white'
					: 'rounded-bl-md bg-bubble text-ink'}"
			>
				{m.text}
			</p>
			{#if m.source}
				<p class="px-2 text-[0.9375rem] text-mute">{m.source}</p>
			{/if}
		</div>
	{/each}
</div>
