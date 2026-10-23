<script lang="ts" module>
  const ring = '<rect class="s" x="1.75" y="1.75" width="12.5" height="12.5" rx="5"/>';
  const solid = '<rect class="f" x="1" y="1" width="14" height="14" rx="5.5"/>';
  const bars = (on: number) =>
    '<rect class="sc" x="1.75" y="3" width="12.5" height="10" rx="2.6"/>' +
    [4.3, 7, 9.7]
      .map((x, i) => `<rect class="${i < on ? 'fc' : 'fc off'}" x="${x}" y="5.4" width="2" height="5.2" rx="0.7"/>`)
      .join('');

  const glyphs: Record<string, string> = {
    'status-done': solid + '<path class="os" d="M5 8.2l2.1 2.1 4-4.5"/>',
    'status-review':
      ring + '<path class="f" d="M4.4 7.3a2.6 2.6 0 0 1 2.6-2.6h2a2.6 2.6 0 0 1 2.6 2.6v1.4a2.6 2.6 0 0 1-2.6 2.6h-2a2.6 2.6 0 0 1-2.6-2.6z"/>',
    'status-progress': ring + '<path class="f" d="M4.4 8.2h7.2v.5a2.6 2.6 0 0 1-2.6 2.6h-2a2.6 2.6 0 0 1-2.6-2.6z"/>',
    'status-todo': ring,
    'status-unscoped': '<rect class="s dash" x="1.75" y="1.75" width="12.5" height="12.5" rx="5"/>',
    'status-backlog':
      '<rect class="s dash" x="1.75" y="1.75" width="12.5" height="12.5" rx="5"/><circle class="f" cx="8" cy="8" r="1.6"/>',
    'status-canceled': '<circle class="s" cx="8" cy="8" r="6.4"/><path class="s" d="M3.6 3.6l8.8 8.8"/>',
    'status-duplicate': ring + '<path class="s" d="M6 6l4 4M10 6l-4 4"/>',
    'status-triage': solid + '<path class="os" d="M4.6 8h6.8M6.3 6.2 4.5 8l1.8 1.8M9.7 6.2 11.5 8 9.7 9.8"/>',
    'status-blocked': solid + '<path class="os" d="M5 8h6"/>',
    'status-blocks': solid + '<path class="os" d="M8 4.5v4.1"/><circle class="o" cx="8" cy="11.1" r="1"/>',
    'status-parent': '<path class="sc bold" d="M4.4 11.6 11 5M5.9 4.6h5.5v5.5"/>',
    'status-subissue':
      '<rect class="fc soft" x="2" y="3.6" width="7.6" height="10" rx="1.6" transform="rotate(-12 5.8 8.6)"/><rect class="fc" x="5.6" y="1.9" width="8.2" height="10.6" rx="1.6" transform="rotate(9 9.7 7.2)"/>',
    'priority-urgent':
      '<path class="f" d="M8 1.6c.55 0 1 .3 1.3.8l5.4 9.6c.55 1-.15 2.2-1.3 2.2H2.6c-1.15 0-1.85-1.2-1.3-2.2l5.4-9.6c.3-.5.75-.8 1.3-.8z"/><path class="os" d="M8 5.6v3.6"/><circle class="o" cx="8" cy="11.6" r="0.95"/>',
    'priority-high': bars(3),
    'priority-medium': bars(2),
    'priority-low': bars(1),
    'priority-none': '<path class="sc" d="M3.5 8h9"/>'
  };

  const tones: Record<string, string> = {
    'status-done': 'done',
    'status-review': 'review',
    'status-progress': 'progress',
    'status-todo': 'todo',
    'status-unscoped': 'unscoped',
    'status-backlog': 'backlog',
    'status-canceled': 'canceled',
    'status-duplicate': 'duplicate',
    'status-triage': 'triage',
    'status-blocked': 'blocked',
    'status-blocks': 'blocks',
    'priority-urgent': 'urgent'
  };

  const lines: Record<string, string> = {
    filter: '<path d="M4 7h16M7 12h10M10 17h4"/>',
    'chevron-down': '<path d="m6.5 9.5 5.5 5.5 5.5-5.5"/>',
    'chevron-up': '<path d="m6.5 14.5 5.5-5.5 5.5 5.5"/>',
    'chevron-right': '<path d="m9.5 6.5 5.5 5.5-5.5 5.5"/>',
    'chevron-left': '<path d="m14.5 6.5-5.5 5.5 5.5 5.5"/>',
    'arrow-left': '<path d="M19.5 12h-15m6.5-6.5L4.5 12l6.5 6.5"/>',
    'arrow-right': '<path d="M4.5 12h15M13 5.5l6.5 6.5-6.5 6.5"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    x: '<path d="m6.5 6.5 11 11M17.5 6.5l-11 11"/>',
    search: '<circle cx="10.8" cy="10.8" r="6.3"/><path d="m15.5 15.5 4.5 4.5"/>',
    more: '<circle class="dot" cx="5.5" cy="12" r="1.5"/><circle class="dot" cx="12" cy="12" r="1.5"/><circle class="dot" cx="18.5" cy="12" r="1.5"/>',
    sidebar: '<rect x="3.5" y="4.5" width="17" height="15" rx="3.2"/><path d="M9.5 4.5v15"/>',
    grid: '<rect x="4" y="4" width="6.6" height="6.6" rx="2.2"/><rect x="13.4" y="4" width="6.6" height="6.6" rx="2.2"/><rect x="4" y="13.4" width="6.6" height="6.6" rx="2.2"/><rect x="13.4" y="13.4" width="6.6" height="6.6" rx="2.2"/>',
    inbox: '<rect x="3.5" y="4.5" width="17" height="15" rx="3.2"/><path d="M3.5 13h4.6l1.4 2.4h5l1.4-2.4h4.6"/>',
    note: '<path d="M7 3.5h6.8l4.7 4.7V18a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 18V6A2.5 2.5 0 0 1 7 3.5z"/><path d="M13.5 3.7v4.8h4.8"/>',
    help: '<circle cx="12" cy="12" r="8.6"/><path d="M9.6 9.6a2.5 2.5 0 0 1 4.85.85c0 1.65-2.45 2-2.45 3.6"/><circle class="dot" cx="12" cy="17" r="1.1"/>',
    compose: '<rect x="3.5" y="3.5" width="17" height="17" rx="4.5"/><path d="M12 8.4v7.2M8.4 12h7.2"/>',
    'user-square': '<rect x="3.5" y="3.5" width="17" height="17" rx="4.5"/><circle cx="12" cy="10.2" r="2.7"/><path d="M7.6 17.4c.9-1.9 2.5-2.9 4.4-2.9s3.5 1 4.4 2.9"/>',
    tag: '<path d="M3.8 5.2v5.6c0 .6.2 1.1.7 1.5l7.7 7.7c.8.8 2 .8 2.8 0l5.3-5.3c.8-.8.8-2 0-2.8l-7.7-7.7c-.4-.5-.9-.7-1.5-.7H5.2c-.8 0-1.4.6-1.4 1.4z"/><circle class="dot" cx="8.3" cy="8.6" r="1.4"/>',
    layers: '<path d="M12 3.8 3.6 8.2l8.4 4.4 8.4-4.4z"/><path d="m3.6 12.1 8.4 4.4 8.4-4.4"/><path d="m3.6 16 8.4 4.4 8.4-4.4"/>',
    list: '<path d="M9.5 6.5h10M9.5 12h10M9.5 17.5h10"/><circle class="dot" cx="5" cy="6.5" r="1.1"/><circle class="dot" cx="5" cy="12" r="1.1"/><circle class="dot" cx="5" cy="17.5" r="1.1"/>',
    rows: '<path d="M4.5 6.5h15M4.5 12h15M4.5 17.5h15"/>',
    columns: '<rect x="3.5" y="4.5" width="17" height="15" rx="3.2"/><path d="M9.2 4.5v15M14.8 4.5v15"/>',
    settings:
      '<path d="M19.06 10.57L21.52 10.76L21.52 13.24L19.06 13.43L18.00 15.98L19.61 17.85L17.85 19.61L15.98 18.00L13.43 19.06L13.24 21.52L10.76 21.52L10.57 19.06L8.02 18.00L6.15 19.61L4.39 17.85L6.00 15.98L4.94 13.43L2.48 13.24L2.48 10.76L4.94 10.57L6.00 8.02L4.39 6.15L6.15 4.39L8.02 6.00L10.57 4.94L10.76 2.48L13.24 2.48L13.43 4.94L15.98 6.00L17.85 4.39L19.61 6.15L18.00 8.02Z"/><circle cx="12" cy="12" r="3"/>',
    bell: '<path d="M6.2 16.3v-5.1a5.8 5.8 0 0 1 11.6 0v5.1l1.6 2.2H4.6z"/><path d="M10 20.5a2.1 2.1 0 0 0 4 0"/>',
    check: '<path d="m5.5 12.5 4.2 4.2 8.8-9.4"/>',
    eye: '<path d="M2.6 12s3.4-6.4 9.4-6.4 9.4 6.4 9.4 6.4-3.4 6.4-9.4 6.4S2.6 12 2.6 12z"/><circle cx="12" cy="12" r="2.9"/>',
    send: '<path d="M20.3 3.7 10.6 13.4"/><path d="m20.3 3.7-6.1 16.6-3.6-6.9-6.9-3.6z"/>',
    refresh: '<path d="M19.4 12.6a7.5 7.5 0 0 1-13 4.5"/><path d="M4.6 11.4a7.5 7.5 0 0 1 13-4.5"/><path d="M17.8 3.6v3.6h-3.6"/><path d="M6.2 20.4v-3.6h3.6"/>',
    edit: '<path d="M11.2 4.6H6.6a2.1 2.1 0 0 0-2.1 2.1v10.7c0 1.2.9 2.1 2.1 2.1h10.7c1.2 0 2.1-.9 2.1-2.1v-4.6"/><path d="M17.5 4a2 2 0 0 1 2.9 2.9l-7.9 7.9-3.7.9.9-3.7z"/>',
    camera: '<path d="M4 8.6c0-1.1.9-2 2-2h2.1L9.6 4h4.8l1.5 2.6H18c1.1 0 2 .9 2 2v8.6c0 1.1-.9 2-2 2H6c-1.1 0-2-.9-2-2z"/><circle cx="12" cy="12.6" r="3.4"/>',
    mail: '<rect x="3.5" y="5.5" width="17" height="13" rx="2.8"/><path d="m4.2 7.2 7.8 5.9 7.8-5.9"/>',
    calendar: '<rect x="4" y="5" width="16" height="15" rx="3.2"/><path d="M4 10h16M8.5 3v4M15.5 3v4"/><circle class="dot" cx="8.6" cy="14.2" r="1"/><circle class="dot" cx="12" cy="14.2" r="1"/><circle class="dot" cx="15.4" cy="14.2" r="1"/><circle class="dot" cx="8.6" cy="17.1" r="1"/><circle class="dot" cx="12" cy="17.1" r="1"/>',
    document: '<path d="M7 3.5h6.6l4.9 4.9V18a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 18V6A2.5 2.5 0 0 1 7 3.5z"/><path d="M13.2 3.7v5h5"/><path d="M8.5 13h7M8.5 16.5h4.5"/>',
    'document-scan': '<path d="M7.5 3.5h5.8l4.2 4.2v9.8a2.5 2.5 0 0 1-2.5 2.5H7.5A2.5 2.5 0 0 1 5 17.5V6a2.5 2.5 0 0 1 2.5-2.5z"/><path d="M3 12h18"/>',
    hourglass: '<path d="M6.5 3.5h11M6.5 20.5h11"/><path d="M7.6 3.5c0 4.2 4.4 4.8 4.4 8.5s-4.4 4.3-4.4 8.5M16.4 3.5c0 4.2-4.4 4.8-4.4 8.5s4.4 4.3 4.4 8.5"/>',
    folder: '<path d="M3.5 7.2c0-1.2.9-2.1 2.1-2.1h3.9l2.1 2.5h6.8c1.2 0 2.1.9 2.1 2.1v7.1c0 1.2-.9 2.1-2.1 2.1H5.6a2.1 2.1 0 0 1-2.1-2.1z"/>',
    chat: '<path d="M12 4.2c4.7 0 8.5 3.3 8.5 7.4s-3.8 7.4-8.5 7.4c-1.1 0-2.1-.2-3-.5l-4.5 1.6 1.2-3.7C4.4 15 3.5 13.4 3.5 11.6c0-4.1 3.8-7.4 8.5-7.4z"/><path d="M8.6 10.4h6.8M8.6 13.4h4"/>',
    code: '<path d="m8.4 8-4 4 4 4M15.6 8l4 4-4 4M13.4 6.2l-2.8 11.6"/>',
    receipt: '<path d="M6 3.5h12v17l-2-1.3-2 1.3-2-1.3-2 1.3-2-1.3-2 1.3z"/><path d="M9 8.2h6M9 11.7h6M9 15.2h3.5"/>',
    home: '<path d="M4.5 10.5 12 4.2l7.5 6.3V18a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 18z"/><path d="M10 20.5v-5.5h4v5.5"/>',
    building: '<rect x="5" y="3.5" width="14" height="17" rx="2.4"/><path d="M9 8h1.5M13.5 8H15M9 12h1.5M13.5 12H15M10.5 20.5V17h3v3.5"/>',
    database: '<ellipse cx="12" cy="6.5" rx="7" ry="3"/><path d="M5 6.5v11c0 1.7 3.1 3 7 3s7-1.3 7-3v-11M5 12c0 1.7 3.1 3 7 3s7-1.3 7-3"/>',
    link: '<path d="M10.5 13.5a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1"/><path d="M13.5 10.5a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1"/>',
    sun: '<circle cx="12" cy="12" r="3.8"/><path d="M12 3v1.8M12 19.2V21M3 12h1.8M19.2 12H21M5.6 5.6l1.3 1.3M17.1 17.1l1.3 1.3M5.6 18.4l1.3-1.3M17.1 6.9l1.3-1.3"/>',
    moon: '<path d="M19.5 14.2A7.8 7.8 0 0 1 9.8 4.5a7.8 7.8 0 1 0 9.7 9.7z"/>',
    swatch: '<rect x="3.5" y="3.5" width="7.5" height="17" rx="2.5"/><path d="M11 8.8 15.4 4.4a2 2 0 0 1 2.8 0l1.4 1.4a2 2 0 0 1 0 2.8L11 17.2"/><path d="M11 20.5h7a2.5 2.5 0 0 0 2.5-2.5v-1.5a2.5 2.5 0 0 0-2.5-2.5h-2"/>',
    spinner: '<path class="spin" d="M12 3.5a8.5 8.5 0 1 1-8.5 8.5"/>'
  };

  const solids: Record<string, [string, string]> = {
    sparkle: [
      '0 0 24 24',
      '<path class="ai" d="M10 3.2c.5 4.3 2.4 6.3 6.6 6.8-4.2.5-6.1 2.5-6.6 6.8-.5-4.3-2.4-6.3-6.6-6.8 4.2-.5 6.1-2.5 6.6-6.8z"/><path class="ai" d="M17.6 13.6c.3 2.2 1.2 3.2 3.4 3.5-2.2.3-3.1 1.3-3.4 3.5-.3-2.2-1.2-3.2-3.4-3.5 2.2-.3 3.1-1.3 3.4-3.5z"/>'
    ],
    brand: [
      '0 0 24 24',
      '<rect class="fc" x="2.5" y="3" width="19" height="6" rx="2"/><path class="fc" d="M4 10.4h16v7.4a2.7 2.7 0 0 1-2.7 2.7H6.7A2.7 2.7 0 0 1 4 17.8z"/><rect class="cut" x="9.4" y="13" width="5.2" height="1.9" rx="0.95"/>'
    ],
  };

  export const iconNames = [...Object.keys(lines), ...Object.keys(solids)];
  export const glyphNames = Object.keys(glyphs);
</script>

<script lang="ts">
  let {
    name,
    size = 'md',
    mono = false,
    label
  }: { name: string; size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'; mono?: boolean; label?: string } = $props();

  const kind = $derived(name in glyphs ? 'glyph' : name in solids ? 'solid' : 'line');
  const viewBox = $derived(kind === 'glyph' ? '0 0 16 16' : kind === 'solid' ? solids[name][0] : '0 0 24 24');
  const body = $derived(kind === 'glyph' ? glyphs[name] : kind === 'solid' ? solids[name][1] : (lines[name] ?? ''));
</script>

<svg
  class="icon"
  data-kind={kind}
  data-size={size}
  data-tone={tones[name]}
  data-mono={mono || undefined}
  data-name={name}
  {viewBox}
  role={label ? 'img' : undefined}
  aria-label={label}
  aria-hidden={label ? undefined : 'true'}
>
  {@html body}
</svg>

<style>
  .icon {
    flex: none;
    display: block;
    width: var(--icon-size);
    height: var(--icon-size);
    overflow: visible;
    --glyph: currentColor;
    --glyph-on: var(--status-glyph-foreground);
  }
  .icon[data-size='xs'] {
    width: var(--icon-size-xs);
    height: var(--icon-size-xs);
  }
  .icon[data-size='sm'] {
    width: var(--icon-size-sm);
    height: var(--icon-size-sm);
  }
  .icon[data-size='lg'] {
    width: var(--icon-size-lg);
    height: var(--icon-size-lg);
  }
  .icon[data-size='xl'] {
    width: var(--icon-size-xl);
    height: var(--icon-size-xl);
  }
  .icon[data-tone='done'] { --glyph: var(--status-done); }
  .icon[data-tone='review'] { --glyph: var(--status-review); }
  .icon[data-tone='progress'] { --glyph: var(--status-progress); }
  .icon[data-tone='todo'] { --glyph: var(--status-todo); }
  .icon[data-tone='unscoped'] { --glyph: var(--status-unscoped); }
  .icon[data-tone='backlog'] { --glyph: var(--status-backlog); }
  .icon[data-tone='canceled'] { --glyph: var(--status-canceled); }
  .icon[data-tone='duplicate'] { --glyph: var(--status-duplicate); }
  .icon[data-tone='triage'] { --glyph: var(--status-triage); }
  .icon[data-tone='blocked'] { --glyph: var(--status-blocked); }
  .icon[data-tone='blocks'] { --glyph: var(--status-blocks); }
  .icon[data-tone='urgent'] { --glyph: var(--status-urgent); }
  .icon[data-mono] {
    --glyph: currentColor;
    --glyph-on: var(--glyph-mono-on, var(--status-glyph-foreground));
  }

  .icon[data-kind='line'] :global(*) {
    fill: none;
    stroke: currentColor;
    stroke-width: var(--icon-stroke);
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .icon[data-kind='line'] :global(.dot) {
    fill: currentColor;
    stroke: none;
  }
  .icon :global(.f) { fill: var(--glyph); }
  .icon :global(.s) {
    fill: none;
    stroke: var(--glyph);
    stroke-width: 1.5;
    stroke-linecap: round;
  }
  .icon :global(.dash) { stroke-dasharray: 2.05 2.05; }
  .icon :global(.o) { fill: var(--glyph-on); }
  .icon :global(.os) {
    fill: none;
    stroke: var(--glyph-on);
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .icon :global(.fc) { fill: currentColor; }
  .icon :global(.sc) {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .icon :global(.thin) { stroke-width: 1; }
  .icon :global(.bold) { stroke-width: 1.9; }
  .icon :global(.soft) { opacity: var(--opacity-soft); }
  .icon :global(.off) { opacity: var(--opacity-inactive-bar); }
  .icon :global(.ai) { fill: var(--ai-strong); }
  .icon :global(.cut) { fill: var(--glyph-cut, var(--card)); }
  .icon :global(.spin) {
    transform-origin: 50% 50%;
    animation: icon-spin var(--duration-slower) linear infinite;
  }
  @keyframes icon-spin {
    to {
      transform: rotate(1turn);
    }
  }
</style>
