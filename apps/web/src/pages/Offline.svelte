<script lang="ts">
  import { AuthFrame, Button, Text } from '../lib';
  import { load, session } from '../lib/session.svelte';
  let busy = $state(false);
</script>

<AuthFrame title="Archive is not reachable" description="The dashboard could not reach the Archive server.">
  <Text as="p" variant="muted">Check that the server is running and that this address points at it, then try again.</Text>
  <Button
    variant="primary"
    size="lg"
    block
    loading={busy}
    onclick={async () => {
      busy = true;
      session.status = 'loading';
      await load();
      busy = false;
    }}>Try again</Button
  >
</AuthFrame>
