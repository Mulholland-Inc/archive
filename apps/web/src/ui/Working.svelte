<script>
  import Bot from './Bot.svelte';

  let { seed = '', kind = 'personal' } = $props();

  const SHARED = ['Checking the stacks', 'Pulling the file', 'Reading the fine print', 'Cross-referencing', 'Blowing off the dust'];
  const LINES = {
    personal: ['Going through your things', 'Opening the old box'],
    family: ['Turning the album pages', 'Climbing the family tree', 'Opening the old trunk'],
    business: ['Opening the ledger', 'Going through the books', 'Checking the minutes'],
  };
  const lines = $derived([...SHARED, ...(LINES[kind] ?? [])]);
  const pick = () => Math.floor(Math.random() * 1000);
  let at = $state(pick());

  $effect(() => {
    const timer = setInterval(() => (at += 1 + (pick() % 3)), 2600);
    return () => clearInterval(timer);
  });
</script>

<div class="working" role="status" aria-label="Working">
  <span class="character"><Bot {seed} size={30} /></span>
  {#key at}<span class="line">{lines[at % lines.length]}</span>{/key}
</div>

<style>
  .working { display: flex; align-items: center; gap: var(--space-2); align-self: flex-start; padding: var(--space-1) 0; }
  .character { display: grid; flex: none; animation: bob 2.2s ease-in-out infinite; }
  .line { background: linear-gradient(100deg, var(--fg-muted) 35%, var(--fg) 50%, var(--fg-muted) 65%) 100% 0 / 260% 100%; background-clip: text; -webkit-background-clip: text; color: transparent; animation: shimmer 2.4s linear infinite, arrive 0.3s ease-out; }
  @keyframes bob { 0%, 100% { transform: translateY(0) rotate(-2deg); } 50% { transform: translateY(-3px) rotate(2deg); } }
  @keyframes shimmer { to { background-position: -60% 0; } }
  @keyframes arrive { from { opacity: 0; transform: translateY(3px); } }
  @media (prefers-reduced-motion: reduce) {
    .character { animation: none; }
    .line { animation: none; background: none; color: var(--fg-muted); }
  }
</style>
