<script>
  import { Copy, Check } from '@lucide/svelte';
  import Icon from './Icon.svelte';

  let { code = '', lang = 'text' } = $props();
  let copied = $state(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(code);
    } catch {}
    copied = true;
    setTimeout(() => (copied = false), 1600);
  }
</script>

<figure class="code">
  <figcaption>
    <span>{lang}</span>
    <button type="button" class="copy" onclick={copy} aria-label="Copy code">
      <Icon icon={copied ? Check : Copy} size="sm" />
      <span>{copied ? 'Copied' : 'Copy'}</span>
    </button>
  </figcaption>
  <pre><code>{code}</code></pre>
</figure>

<style>
  .code {
    display: flex;
    flex-direction: column;
    min-width: 0;
    border-radius: var(--radius-lg);
    background: var(--color-code);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border-subtle);
    overflow: hidden;
  }

  figcaption {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-1-5);
    color: var(--color-text-tertiary);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    letter-spacing: var(--tracking-normal);
  }

  figcaption > span {
    padding: var(--space-1-5);
  }

  .copy {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    height: var(--control-xs);
    padding: var(--space-1-5);
    border-radius: var(--radius-sm);
    color: var(--color-text-tertiary);
    font-size: var(--text-xs);
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .copy:hover {
    background: var(--color-fill);
    color: var(--color-text);
  }

  pre {
    max-height: var(--code-max);
    overflow: auto;
    padding: var(--space-4);
    padding-block-start: var(--space-0);
    font-family: var(--font-mono);
    font-size: var(--text-code);
    line-height: var(--leading-relaxed);
    letter-spacing: var(--tracking-normal);
    color: var(--color-text);
  }
</style>
