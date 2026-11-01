<script>
  import { ArrowUpRight } from '@lucide/svelte';
  import Stack from './Stack.svelte';
  import Section from './Section.svelte';
  import Divider from './Divider.svelte';
  import SettingsRow from './SettingsRow.svelte';
  import Button from './Button.svelte';
  import Chip from './Chip.svelte';
  import Text from './Text.svelte';
  import PaneHeader from './PaneHeader.svelte';

  const cloud = 'https://console.cloud.google.com/home/dashboard?project=mulholland-archive';

  const sections = [
    {
      title: 'Hosting',
      rows: [
        ['Status', { chip: 'Healthy' }],
        ['Provider', 'Google Cloud'],
        ['Project', 'mulholland-archive'],
        ['Region', 'us-central1 (Iowa)'],
        ['Version', '0.42.1, commit 360a06b']
      ]
    },
    {
      title: 'Usage today',
      rows: [
        ['Requests', '18,402'],
        ['Workflow runs', '326'],
        ['Storage', '98.6 GB']
      ]
    },
    {
      title: 'API',
      rows: [
        ['Status', { chip: 'Healthy' }],
        ['Runs on', 'Cloud Run'],
        ['Service', 'archive-api'],
        ['Revision', 'archive-api-00142-xuz'],
        ['Instances', '2 running, scales to zero'],
        ['Min and max instances', '0 to 10'],
        ['Cold start', '380 ms'],
        ['Last deploy', 'Today at 9:41 AM']
      ]
    },
    {
      title: 'Database',
      rows: [
        ['Status', { chip: 'Healthy' }],
        ['Engine', 'Cloud SQL, Postgres 18'],
        ['Tier', 'db-custom-2-8192'],
        ['Storage used', '12.4 GB of 50 GB'],
        ['Backups', 'Daily at 3:00 AM'],
        ['Point-in-time recovery', { chip: 'On' }]
      ]
    },
    {
      title: 'Files',
      rows: [
        ['Stored in', 'Cloud Storage'],
        ['Bucket', 'mulholland-archive-files'],
        ['Size used', '86.2 GB'],
        ['Objects', '48,213']
      ]
    },
    {
      title: 'Background work',
      rows: [
        ['Runs on', 'Cloud Tasks'],
        ['Queue', 'archive-due'],
        ['Tasks today', '1,284']
      ]
    },
    {
      title: 'Domains',
      rows: [
        ['app.archive.dev', 'Static Worker on Cloudflare'],
        ['api.archive.dev', 'Cloud Run behind Cloudflare proxy']
      ]
    }
  ];
</script>

<PaneHeader title="Server" description="Where this vault runs, what it uses and how healthy it is right now." art="server" />
<Stack gap="6">
  {#each sections as section, i (section.title)}
    {#if i}<Divider />{/if}
    <Section title={section.title}>
      {#snippet actions()}
        {#if !i}<Button size="sm" href={cloud} target="_blank" rel="noreferrer" iconEnd={ArrowUpRight}>View in Google Cloud</Button>{/if}
      {/snippet}
      <Stack gap="0">
        {#each section.rows as [label, value] (label)}
          <SettingsRow {label}>
            {#if typeof value === 'string'}<Text tone="secondary" truncate>{value}</Text>{:else}<Chip size="sm">{value.chip}</Chip>{/if}
          </SettingsRow>
        {/each}
      </Stack>
    </Section>
  {/each}
</Stack>
