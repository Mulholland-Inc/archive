<script>
  import { page } from '$app/state';
  import { afterNavigate } from '$app/navigation';
  import Art from './Art.svelte';
  import Button from './Button.svelte';
  import Word from './Word.svelte';
  import { APP, GITHUB, PRODUCT } from './site.js';

  const LINKS = [
    ['/developers', 'Developers'],
    ['/pricing', 'Pricing'],
    ['/docs', 'Docs'],
    ['/blog', 'Blog'],
    ['/changelog', 'Changelog']
  ];

  let open = $state(false);
  let shut = $state(false);
  let y = $state(0);
  let menu;

  const here = $derived(page.url.pathname.replace(/(.)\/$/, '$1'));

  afterNavigate(({ from }) => {
    open = false;
    if (!from) return;
    if (menu.matches(':hover')) shut = true;
    if (menu.contains(document.activeElement)) document.activeElement.blur();
  });
</script>

<svelte:window bind:scrollY={y} />

<header class={['nav', y > 8 && 'scrolled']}>
  <div class="wrap">
    <a class="brand" href="/" aria-label="Archive home"><Word /></a>
    <nav class="nav-links">
      <div class="menu" bind:this={menu} role="presentation" onmouseleave={() => (shut = false)}>
        <button type="button" aria-haspopup="true"
          >Product <svg viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 3.5 5 6.5 8 3.5" /></svg></button
        >
        <div class="menu-panel" style:visibility={shut ? 'hidden' : null} style:opacity={shut ? '0' : null}>
          <div class="menu-grid">
            {#each PRODUCT as p}
              <a href={p.href}><span class="mi-art"><Art name={p.art} lazy /></span><strong>{p.title}</strong><small>{p.text}</small></a>
            {/each}
          </div>
        </div>
      </div>
      {#each LINKS as [href, label]}
        <a {href} aria-current={here === href ? 'page' : undefined}>{label}</a>
      {/each}
    </nav>
    <div class="nav-right">
      <a class="icon-btn gh" href={GITHUB} target="_blank" rel="noopener" aria-label="GitHub"
        ><svg viewBox="0 0 16 16" fill="currentColor"
          ><path
            d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
          /></svg
        ></a
      >
      <Button href={APP} small>Log in</Button>
      <button class="icon-btn burger" type="button" aria-label="Menu" onclick={() => (open = !open)}
        ><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 5h12M2 11h12" /></svg></button
      >
    </div>
  </div>
</header>
<div class={['sheet', open && 'open']}>
  {#each PRODUCT as p}<a href={p.href}>{p.title}</a>{/each}
  <a href="/pricing">Pricing</a><a href="/docs">Docs</a><a href="/blog">Blog</a><a href="/changelog">Changelog</a><a href={GITHUB}>GitHub</a><a href={APP}>Log in</a>
</div>
