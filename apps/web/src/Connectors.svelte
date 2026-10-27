<script>
  import { api } from './api.js';
  import Button from './ui/Button.svelte';
  import Card from './ui/Card.svelte';
  import Copy from './ui/Copy.svelte';
  import Grid from './ui/Grid.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';

  let { space } = $props();
  const origin = location.origin;
  let token = $state('');
  let busy = $state(false);
  let error = $state('');
  const bearer = $derived(token || '<token>');

  const ways = $derived([
    { icon: 'bot', name: 'Claude', line: 'Desktop and Code reach your archive through MCP.', copy: `claude mcp add --transport http archive ${origin}/mcp --header "Authorization: Bearer ${bearer}"` },
    { icon: 'messages-square', name: 'ChatGPT and other MCP clients', line: 'Streamable HTTP with your token as a bearer.', copy: `${origin}/mcp` },
    { icon: 'braces', name: 'HTTP API', line: 'JSON over REST for every table, search and files.', copy: `curl -H "Authorization: Bearer ${bearer}" ${origin}/api/types` },
    { icon: 'terminal', name: 'Command line', line: 'On the machine that runs Archive, or MCP over stdio.', copy: 'archive mcp' },
    { icon: 'boxes', name: 'Terraform', line: 'Schema, tokens and members as code. Needs a full token.', copy: `provider "archive" { endpoint = "${origin}" }` },
  ]);

  const path = $derived(`/auth/spaces/${encodeURIComponent(space)}/token`);

  async function load(rotate) {
    busy = true;
    error = '';
    try {
      token = (await api(rotate ? `${path}/rotate` : path, rotate ? { method: 'POST' } : {})).token;
    } catch (e) {
      error = e.message;
    }
    busy = false;
  }

  load(false);
</script>

<Heading title="Connectors" icon="plug" />
<Grid>
  <Card icon="key-round" title="Token">
    {#if token}
      <Copy text={token} />
      <div><Button icon="refresh-cw" onclick={() => load(true)} disabled={busy}>{busy ? 'Rotating…' : 'Rotate'}</Button></div>
    {/if}
    {#if error}<Note error>{error}</Note>{/if}
  </Card>
  {#each ways as w (w.name)}
    <Card icon={w.icon} title={w.name} details={[w.line]}>
      <Copy text={w.copy} />
    </Card>
  {/each}
</Grid>
