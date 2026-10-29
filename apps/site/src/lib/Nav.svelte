<script>
  import { page } from '$app/state';
  import { afterNavigate } from '$app/navigation';
  import Art from './Art.svelte';
  import Button from '#kit/Button.svelte';
  import Icon from '#kit/Icon.svelte';
  import Theme from '#kit/Theme.svelte';
  import Word from '#kit/Word.svelte';
  import { APP, BUILT, GITHUB, PRODUCT } from './site.js';

  const LINKS = [
    ['/developers', 'Developers'],
    ['/pricing', 'Pricing'],
    ['/docs', 'Docs'],
    ['/blog', 'Blog']
  ];

  let open = $state(false);
  let shut = $state(false);
  let hidden = $state(false);
  let header;
  let menu;
  let last = 0;
  let slack = 0;

  const here = $derived(page.url.pathname.replace(/(.)\/$/, '$1'));

  $effect(() => {
    document.documentElement.classList.toggle('locked', open);
  });

  $effect(() => {
    slack =parseFloat(getComputedStyle(header).getPropertyValue('--nav-slack'));
  });

  function scrolled() {
    const y = Math.min(Math.max(scrollY, 0), document.documentElement.scrollHeight - innerHeight);
    if (y <= header.offsetHeight || open || menu.matches(':hover, :focus-within')) {
      hidden = false;
      last = y;
      return;
    }
    if (Math.abs(y - last) < slack) return;
    hidden = y > last;
    last = y;
  }

  afterNavigate(({ from }) => {
    open = false;
    hidden = false;
    last = scrollY;
    if (!from) return;
    if (menu.matches(':hover')) shut = true;
    if (menu.contains(document.activeElement)) document.activeElement.blur();
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (open = false)} onscroll={scrolled} />

<header class={['nav', open && 'open', hidden && 'hidden']} bind:this={header} onfocusin={() => (hidden = false)}>
  <div class="wrap">
    <a class="brand" href="/" aria-label="Archive home"><Word /></a>
    <nav class="nav-links">
      <div class="menu" bind:this={menu} role="presentation" onmouseleave={() => (shut = false)}>
        <button type="button" aria-haspopup="true">Product <Icon name="chevron" /></button>
        <div class={['menu-panel', shut && 'shut']}>
          <div class="menu-grid">
            {#each PRODUCT as p}
              <a href={p.href}><span class="mi-art"><Art name={p.art} lazy /></span><strong>{p.title}</strong><small>{p.text}</small></a>
            {/each}
          </div>
          <div class="menu-built">
            <span class="eyebrow">Built on Archive</span>
            {#each BUILT as b}
              <a href={b.href}><span class="mb-art"><Art name={b.art} lazy /></span><strong>{b.title}</strong><small>{b.text}</small><Icon name="arrow" /></a>
            {/each}
          </div>
        </div>
      </div>
      {#each LINKS as [href, label]}
        <a {href} aria-current={here === href ? 'page' : undefined}>{label}</a>
      {/each}
    </nav>
    <div class="nav-right">
      <Theme />
      <a class="icon-btn gh" href={GITHUB} target="_blank" rel="noopener" aria-label="GitHub"><Icon name="github" /></a>
      <Button href={APP} small>Log in</Button>
      <button class="icon-btn burger" type="button" aria-label={open ? 'Close menu' : 'Menu'} aria-expanded={open} aria-controls="sheet" onclick={() => (open = !open)}><Icon name={open ? 'close' : 'menu'} /></button>
    </div>
  </div>
  <div class="sheet" id="sheet" inert={!open}>
    <div class="sheet-grid">
      {#each PRODUCT as p}
        <a href={p.href}><span class="sheet-art"><Art name={p.art} lazy /></span><span>{p.title}</span></a>
      {/each}
      {#each BUILT as b}
        <a class="sheet-built" href={b.href}><span class="sheet-art"><Art name={b.art} lazy /></span><span>{b.title}</span><small>Built on Archive</small></a>
      {/each}
    </div>
    <div class="sheet-links">
      {#each LINKS.filter(([href]) => href !== '/developers') as [href, label]}<a {href}>{label}</a>{/each}
      <a href={GITHUB} target="_blank" rel="noopener">GitHub</a>
    </div>
  </div>
</header>
