<script>
  let { token, label } = $props();
</script>

<div class="swatch">
  <span class={['chip', token]}></span>
  <span class="text">
    <span class="name">{label ?? token}</span>
    <code>--{token}</code>
  </span>
</div>

<style>
  .swatch {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }

  .chip {
    flex: none;
    width: var(--control-xl);
    height: var(--control-xl);
    border-radius: var(--radius-md);
    box-shadow: inset 0 0 0 var(--border-width) var(--color-border);
  }

  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .name {
    font-size: var(--text-md);
    font-weight: var(--weight-medium);
  }

  code {
    color: var(--color-text-tertiary);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    letter-spacing: var(--tracking-normal);
  }

  .color-canvas { background: var(--color-canvas); }
  .color-sidebar { background: var(--color-sidebar); }
  .color-surface { background: var(--color-surface); }
  .color-surface-raised { background: var(--color-surface-raised); }
  .color-surface-sunken { background: var(--color-surface-sunken); }
  .color-bubble { background: var(--color-bubble); }
  .color-code { background: var(--color-code); }
  .color-fill-subtle { background: var(--color-fill-subtle); }
  .color-fill { background: var(--color-fill); }
  .color-fill-strong { background: var(--color-fill-strong); }
  .color-border { background: var(--color-border); }
  .color-border-strong { background: var(--color-border-strong); }
  .color-text { background: var(--color-text); }
  .color-text-secondary { background: var(--color-text-secondary); }
  .color-text-tertiary { background: var(--color-text-tertiary); }
  .color-text-disabled { background: var(--color-text-disabled); }
  .color-inverse { background: var(--color-inverse); }
  .color-primary { background: var(--color-primary); }
  .color-accent { background: var(--color-accent); }
  .color-accent-subtle { background: var(--color-accent-subtle); }
  .color-success { background: var(--color-success); }
  .color-warning { background: var(--color-warning); }
  .color-danger { background: var(--color-danger); }
  .color-avatar-1 { background: var(--color-avatar-1); }
  .color-avatar-2 { background: var(--color-avatar-2); }
  .color-avatar-3 { background: var(--color-avatar-3); }
  .color-avatar-4 { background: var(--color-avatar-4); }
  .color-avatar-5 { background: var(--color-avatar-5); }
  .color-avatar-6 { background: var(--color-avatar-6); }
  .ring-subtle { background: var(--color-surface); box-shadow: var(--ring-subtle); }
  .ring { background: var(--color-surface); box-shadow: var(--ring); }
  .ring-strong { background: var(--color-surface); box-shadow: var(--ring-strong); }
  .radius-xs { background: var(--color-fill-strong); border-radius: var(--radius-xs); }
  .radius-sm { background: var(--color-fill-strong); border-radius: var(--radius-sm); }
  .radius-md { background: var(--color-fill-strong); border-radius: var(--radius-md); }
  .radius-lg { background: var(--color-fill-strong); border-radius: var(--radius-lg); }
  .radius-xl { background: var(--color-fill-strong); border-radius: var(--radius-xl); }
  .radius-2xl { background: var(--color-fill-strong); border-radius: var(--radius-2xl); }
  .radius-full { background: var(--color-fill-strong); border-radius: var(--radius-full); }
</style>
