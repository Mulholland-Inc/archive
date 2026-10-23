<script lang="ts" module>
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
    home: '<path d="M4.5 10.5 12 4.2l7.5 6.3V18a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 18z"/><path d="M10 20.5v-5.5h4v5.5"/>',
    building: '<rect x="5" y="3.5" width="14" height="17" rx="2.4"/><path d="M9 8h1.5M13.5 8H15M9 12h1.5M13.5 12H15M10.5 20.5V17h3v3.5"/>',
    database: '<ellipse cx="12" cy="6.5" rx="7" ry="3"/><path d="M5 6.5v11c0 1.7 3.1 3 7 3s7-1.3 7-3v-11M5 12c0 1.7 3.1 3 7 3s7-1.3 7-3"/>',
    link: '<path d="M10.5 13.5a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1"/><path d="M13.5 10.5a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1"/>',
    sun: '<circle cx="12" cy="12" r="3.8"/><path d="M12 3v1.8M12 19.2V21M3 12h1.8M19.2 12H21M5.6 5.6l1.3 1.3M17.1 17.1l1.3 1.3M5.6 18.4l1.3-1.3M17.1 6.9l1.3-1.3"/>',
    moon: '<path d="M19.5 14.2A7.8 7.8 0 0 1 9.8 4.5a7.8 7.8 0 1 0 9.7 9.7z"/>',
    spinner: '<path class="spin" d="M12 3.5a8.5 8.5 0 1 1-8.5 8.5"/>',
    upload: '<path d="M12 15.5V4.5M7.5 9 12 4.5 16.5 9"/><path d="M4.5 15v2.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15"/>',
    download: '<path d="M12 4.5v11M7.5 11 12 15.5 16.5 11"/><path d="M4.5 15v2.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15"/>',
    play: '<path d="M8 5.6v12.8c0 .8.9 1.3 1.6.8l9.4-6.4c.6-.4.6-1.2 0-1.6L9.6 4.8C8.9 4.3 8 4.8 8 5.6z"/>',
    trash: '<path d="M4.5 7h15M9.5 7V5.2c0-.6.5-1.2 1.2-1.2h2.6c.7 0 1.2.6 1.2 1.2V7M6.5 7l.8 11.4c.1 1.2 1 2.1 2.2 2.1h5c1.2 0 2.1-.9 2.2-2.1L17.5 7"/>',
    key: '<circle cx="8.5" cy="15.5" r="4"/><path d="m11.4 12.6 8.1-8.1M16.5 7.5l2.5 2.5M14.5 9.5l2 2"/>',
    user: '<circle cx="12" cy="8.5" r="3.8"/><path d="M4.8 20c1.1-3.6 3.9-5.5 7.2-5.5s6.1 1.9 7.2 5.5"/>',
    users: '<circle cx="9" cy="8.8" r="3.3"/><path d="M3.2 19.5c.9-3.1 3.1-4.8 5.8-4.8s4.9 1.7 5.8 4.8"/><path d="M15.2 5.8a3.3 3.3 0 0 1 0 6.1M17.4 14.9c1.6.6 2.8 2.1 3.4 4.6"/>',
    external: '<path d="M13.5 4.5h6v6M19.5 4.5l-8 8"/><path d="M18 14v3.5a2 2 0 0 1-2 2H6.5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2H10"/>',
    copy: '<rect x="8.5" y="8.5" width="11" height="11" rx="2.4"/><path d="M15.5 8.5V6.5a2 2 0 0 0-2-2h-7a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h2"/>',
    logout: '<path d="M14 4.5H7a2.5 2.5 0 0 0-2.5 2.5v10A2.5 2.5 0 0 0 7 19.5h7"/><path d="M10.5 12h9M16 8.5l3.5 3.5-3.5 3.5"/>',
    plug: '<path d="M9 3.5v4M15 3.5v4"/><path d="M6.5 7.5h11V11a5.5 5.5 0 0 1-11 0z"/><path d="M12 16.5v4"/>',
    shapes: '<rect x="4" y="4" width="7" height="7" rx="1.8"/><circle cx="16.5" cy="7.5" r="3.5"/><path d="M7.5 13.5 11 20H4z"/><rect x="13" y="13" width="7" height="7" rx="3.5"/>',
    history: '<path d="M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9"/><path d="M4.5 4.5V9H9"/><path d="M12 8v4.2l2.8 1.8"/>',
    lock: '<rect x="5" y="10.5" width="14" height="9.5" rx="2.6"/><path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5"/>',
    image: '<rect x="3.5" y="4.5" width="17" height="15" rx="3"/><circle cx="9" cy="10" r="1.8"/><path d="m4.5 18 5-5 3.5 3.5 2.5-2.5 4 4"/>',
    film: '<rect x="3.5" y="4.5" width="17" height="15" rx="3"/><path d="M8 4.5v15M16 4.5v15M3.5 9.5H8M3.5 14.5H8M16 9.5h4.5M16 14.5h4.5"/>',
    music: '<path d="M9.5 17.5V6l10-2v11.5"/><circle cx="7" cy="17.5" r="2.5"/><circle cx="17" cy="15.5" r="2.5"/>',
    table: '<rect x="3.5" y="4.5" width="17" height="15" rx="3"/><path d="M3.5 9.5h17M3.5 14.5h17M9.5 9.5v10"/>',
    alert: '<path d="M12 4 21 19.5H3z"/><path d="M12 10v4"/><circle class="dot" cx="12" cy="16.8" r="1"/>',
    clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>',
    terminal: '<rect x="3.5" y="4.5" width="17" height="15" rx="3"/><path d="m7.5 9.5 3 2.5-3 2.5M12.5 15h4"/>',
    cloud: '<path d="M7.5 18.5a4 4 0 0 1-.6-8 5.5 5.5 0 0 1 10.6 1.5 3.3 3.3 0 0 1 .5 6.5z"/>',
    "eye-off": '<path d="M4 4l16 16"/><path d="M10.6 6A9.6 9.6 0 0 1 12 5.6c6 0 9.4 6.4 9.4 6.4a16.9 16.9 0 0 1-2.6 3.4M6.5 7.6C4 9.4 2.6 12 2.6 12s3.4 6.4 9.4 6.4c1.6 0 3-.5 4.2-1.1"/><path d="M9.9 10a2.9 2.9 0 0 0 4.1 4.1"/>',
    pencil: '<path d="M15.5 5a2.2 2.2 0 0 1 3.1 3.1L8.5 18.2l-4 1 1-4z"/><path d="m13.5 7 3.1 3.1"/>',
    sparkle: '<path d="M10 3.5c.5 4 2.3 5.9 6.3 6.4-4 .5-5.8 2.4-6.3 6.4-.5-4-2.3-5.9-6.3-6.4 4-.5 5.8-2.4 6.3-6.4z"/><path d="M17.5 14c.3 2 1.1 2.9 3 3.2-1.9.3-2.7 1.2-3 3.2-.3-2-1.1-2.9-3-3.2 1.9-.3 2.7-1.2 3-3.2z"/>',
  };

  const solids: Record<string, [string, string]> = {
    brand: [
      '0 0 24 24',
      '<rect class="fc" x="2.5" y="3" width="19" height="6" rx="2"/><path class="fc" d="M4 10.4h16v7.4a2.7 2.7 0 0 1-2.7 2.7H6.7A2.7 2.7 0 0 1 4 17.8z"/><rect class="cut" x="9.4" y="13" width="5.2" height="1.9" rx="0.95"/>'
    ]
  };

  export const iconNames = [...Object.keys(lines), ...Object.keys(solids)];
</script>

<script lang="ts">
  let { name, size = 'md', label }: { name: string; size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'; label?: string } = $props();

  const solid = $derived(name in solids);
  const viewBox = $derived(solid ? solids[name][0] : '0 0 24 24');
  const body = $derived(solid ? solids[name][1] : (lines[name] ?? ''));
</script>

<svg
  class="icon"
  data-kind={solid ? 'solid' : 'line'}
  data-size={size}
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
  .icon :global(.fc) {
    fill: currentColor;
  }
  .icon :global(.cut) {
    fill: var(--glyph-cut, var(--card));
  }
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
