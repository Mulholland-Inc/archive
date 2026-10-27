<script>
  import { api } from './api.js';
  import Button from './ui/Button.svelte';
  import Card from './ui/Card.svelte';
  import Copy from './ui/Copy.svelte';
  import Grid from './ui/Grid.svelte';
  import Heading from './ui/Heading.svelte';
  import Note from './ui/Note.svelte';

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

  async function create() {
    busy = true;
    error = '';
    try {
      token = (await api('/auth/tokens', { method: 'POST', body: { name: `web ${new Date().toLocaleDateString()}` } })).token;
    } catch (e) {
      error = e.message;
    }
    busy = false;
  }
</script>

<Heading title="Connectors" />
<Grid>
  <Card icon="key-round" title="Token" details={[token ? 'Shown once. Keep it somewhere safe.' : 'A bearer token with your access, for the clients here.']}>
    {#if token}
      <Copy text={token} />
    {:else}
      <div><Button icon="plus" onclick={create} disabled={busy}>{busy ? 'Creating…' : 'Create token'}</Button></div>
    {/if}
    {#if error}<Note error>{error}</Note>{/if}
  </Card>
  {#each ways as w (w.name)}
    <Card icon={w.icon} title={w.name} details={[w.line]}>
      <Copy text={w.copy} />
    </Card>
  {/each}
</Grid>
