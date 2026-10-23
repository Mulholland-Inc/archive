<script lang="ts">
  import { Page, EmptyState, Button } from '../lib';
  import { navigate } from '../lib/router.svelte';
  import { setCrumbs } from '../lib/page.svelte';
  setCrumbs([{ label: 'Not found' }]);
</script>

<Page>
  <EmptyState icon="search" title="This page does not exist" description="The link may be old, or the record was deleted.">
    <Button icon="home" onclick={() => navigate('/')}>Go home</Button>
  </EmptyState>
</Page>
