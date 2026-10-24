<script lang="ts">
  import type { Snippet } from 'svelte';
  import BrandMark from './BrandMark.svelte';
  import { brand } from '../brand.svelte';
  let { title, description, children, footer }: { title: string; description?: string; children: Snippet; footer?: Snippet } = $props();
</script>

<div class="frame">
  <div class="brand"><span class="logo"><BrandMark /></span>{brand.name}</div>
  <main class="card">
    <div class="titles">
      <h1 class="title">{title}</h1>
      {#if description}<p class="description">{description}</p>{/if}
    </div>
    {@render children()}
  </main>
  {#if footer}<div class="footer">{@render footer()}</div>{/if}
</div>

<style>
  .frame {
    min-height: 100vh;
    min-height: 100dvh;
    display: grid;
    grid-template-rows: auto 1fr auto;
    justify-items: center;
    padding: var(--space-6);
    background: var(--background);
  }
  .brand {
    justify-self: start;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-md);
    font-weight: var(--font-semibold);
  }
  .logo {
    display: grid;
    place-items: center;
    width: var(--control-height);
    height: var(--control-height);
    border-radius: var(--radius-md);
    background: var(--brand);
    color: var(--brand-foreground);
  }
  .card {
    align-self: center;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    width: 100%;
    max-width: var(--auth-width);
    padding: var(--space-6);
    border-radius: var(--radius-xl);
    background: var(--card);
    box-shadow: inset 0 0 0 var(--border-width) var(--border);
    animation: card-in var(--duration-slower) var(--ease-out);
  }
  .titles {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .title {
    font-family: var(--font-display);
    font-size: var(--text-xl);
    font-weight: var(--font-semibold);
    letter-spacing: var(--tracking-tight);
    line-height: var(--leading-tight);
  }
  .description {
    color: var(--muted-foreground);
  }
  .footer {
    color: var(--muted-foreground);
    font-size: var(--text-sm);
  }
  @keyframes card-in {
    from {
      opacity: 0;
      transform: translateY(var(--slide)) scale(var(--pop-scale));
    }
  }
  @media (max-width: 40rem) {
    .frame {
      padding: var(--space-4);
    }
    .card {
      padding: var(--space-5);
    }
  }
</style>
