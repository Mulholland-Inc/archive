# site

Marketing site for [archive.us.org](https://archive.us.org). SvelteKit (static adapter) + Tailwind, prerendered with zero client-side JavaScript and self-hosted fonts.

```sh
npm install
npm run dev      # http://localhost:5173
npm run build    # static output in build/
```

Copy lives in `src/lib/content.ts`; the page is `src/routes/+page.svelte`; the hero demo card is `src/lib/Intake.svelte`.

Not wired yet: the waitlist form posts to `/waitlist`, which doesn't exist. Prices on the pricing section are placeholders.
