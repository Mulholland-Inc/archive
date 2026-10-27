# ADR 0002: Connectors

- Status: Proposed
- Date: 2026-10-28
- Deciders: Marijn Brussel

## Summary

Connectors bring a space's data in from the services a business already uses: mail, files, chat, docs, work tracking, CRM, support and finance. They are read-only. Each keeps a delta cursor per connection and syncs only what changed, woken by the provider's change notifications where it can send them and by polling the same delta endpoint where it cannot.

Mulholland registers one app per provider and gets it approved once. The client ids ship in the binary, like Thunderbird or rclone ship theirs, so a self-hosted Archive connects without its owner registering anything. Every client id can be overridden for anyone who wants their own app, and services that issue API tokens can be connected with a pasted token and no app at all.

This replaces "polling connectors" in ADR 0001's "Not doing" list.

## Shape

- **Read-only**: connectors request read scopes only and never write back.
- **In Archive, not in the agent**: syncing is plain Go in the binary with no LLM, like file processing. Agents read what landed and classify it, as in ADR 0001.
- **Per space**: a connection belongs to one space, and what it syncs lands in that space under row-level security.
- **One table**: `archive.connection` holds the provider, the account, the encrypted refresh token or API token (`ARCHIVE_SECRET_KEY`, like secret fields), the delta cursor, the change subscription and its expiry, and the last sync and error.
- **Coordination through Postgres**: a sync takes its connection row with `SELECT … FOR UPDATE SKIP LOCKED`, so any number of instances share the work and a connection syncs on one at a time. A change notification only marks the connection due and returns.

## Signing in without registering apps

| Flow | Used for | How it works on a self-hosted server |
|---|---|---|
| Loopback with paste | Google | The client is a Desktop app, which Google lets redirect to `http://127.0.0.1:<any port>` with no registered redirect URL. With PKCE, the browser lands on localhost; on a remote server that page fails and the user pastes its URL back into Archive, as Claude Code and gcloud do. Google treats Desktop client secrets as public, so shipping them is allowed. |
| Device code | Microsoft | A public client with no redirect URL: Archive shows a code, the user enters it at microsoft.com/devicelogin. |
| Redirect to Archive | Providers that require a registered HTTPS redirect URL | Works only where the redirect URL is the instance's own, so the instance owner brings their own app, or uses a token. |
| Pasted token | Providers with API tokens | The user creates a token in the service and pastes it. No app, no approval. |
| Own app from a manifest | Slack | Archive gives a manifest; the workspace admin creates an internal app from it in one click and pastes its tokens. |

Overrides follow one pattern: `ARCHIVE_CONNECT_<PROVIDER>` holds a JSON `{"client_id","client_secret"}`.

## Sync

- **Cursor first**: every connector starts from the provider's delta API (a history id, page token, sync token, delta link or `updated` timestamp) and stores the next cursor in the same transaction as the rows it wrote. A crash repeats a page, never skips one, so writes are upserts on the provider's id.
- **Push where the provider allows a URL per subscription**: Drive, Google Calendar, Graph, Box, Asana, Linear, GitLab, Stripe, Zendesk, Pipedrive and Exact let each subscription name its own callback. Each instance subscribes with `ARCHIVE_URL/hooks/<provider>` and renews before expiry.
- **Pull where push needs no URL**: Box's event long-poll, Salesforce's Pub/Sub API for Change Data Capture, Slack's Socket Mode and Gmail through a Pub/Sub pull subscription work behind NAT.
- **Poll otherwise**: providers that send all webhooks for an app to one URL (Slack's Events API, Dropbox, HubSpot, Intercom, Xero, QuickBooks, Zoom, Notion, GitHub Apps) cannot reach many self-hosted instances without a relay Mulholland would run. Archive polls their delta APIs instead, and so does any instance without a public HTTPS `ARCHIVE_URL`.
- **Interval**: polling backs off with inactivity, from one minute to an hour, so a quiet account costs little and the instance can scale to zero between polls.
- **Initial sync**: walks the full history once, newest first, so recent data is searchable soonest.

A hosted Archive could later run one relay for the one-URL providers. That is the only place a central service would help, and it is not needed to ship.

## Where data lands

- **Mail and chat messages, calendar events, issues, CRM records, tickets and finance records** become rows in built-in type tables (`email`, `message`, `event`, …), created like any type table by `archive.sql`, so they are listed, searched and shown in the UI like the rest.
- **Files** from Drive, SharePoint, OneDrive, Dropbox, Box and attachments become Archive files, processed and text-extracted as uploads are. Google Docs, Sheets and Slides are exported as docx, xlsx and pptx.
- **Deletes** at the source delete the row or file in Archive.

The table shapes are decided per connector when it is built.

## Connectors

The list a B2B space needs, by area. "Change feed" is what the connector syncs from; "Push" is how changes can wake it.

### Mail and calendar

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| Gmail | Gmail API, `gmail.readonly` (restricted) | `history.list` from a history id | Pub/Sub topic via `users.watch`; poll by default |
| Google Calendar | Calendar API, `calendar.readonly` (sensitive) | `events.list` sync token | `events.watch`, URL per channel |
| Outlook mail | Graph, `Mail.Read` | `messages/delta` per folder | Graph subscription, URL per subscription |
| Outlook calendar | Graph, `Calendars.Read` | `calendarView/delta` | Graph subscription |

### Files

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| Google Drive | Drive API, `drive.readonly` (restricted) | `changes.list` page token | `changes.watch`, URL per channel |
| SharePoint and OneDrive | Graph, `Files.Read.All`, `Sites.Read.All` | `driveItem/delta` per drive | Graph subscription on the drive root |
| Dropbox | Dropbox API, `files.content.read` | `list_folder/continue` cursor | One webhook URL per app; long-poll `list_folder/longpoll` |
| Box | Box API, read | Events stream position | Long-poll; webhooks with URL per webhook |

### Chat

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| Slack | Web API, `channels:history`, `groups:history`, `im:history`, `files:read`, `users:read` | `conversations.history` and `conversations.replies` since the last `ts` | Socket Mode for an internal app; Events API URL is one per app |
| Microsoft Teams | Graph, `ChannelMessage.Read.All`, `Chat.Read` (admin consent) | Channel `messages/delta`; chats by `lastModifiedDateTime` | Graph subscription with encrypted resource data, short expiry |
| Google Chat | Chat API, `chat.messages.readonly` | `spaces.messages.list` by create time | Workspace Events API to Pub/Sub |

### Docs and knowledge

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| Notion | Notion API, read content | `search` sorted by `last_edited_time` | One webhook URL per integration |
| Confluence | Atlassian 3LO, `read:confluence-content.all` | CQL `lastmodified` | Dynamic webhooks for OAuth apps, renewed every 30 days |

### Work tracking and code

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| Jira | Atlassian 3LO, `read:jira-work` | JQL `updated` | Dynamic webhooks, renewed every 30 days |
| Linear | GraphQL, `read` | `updatedAt` filter | Webhooks created by API, URL per webhook |
| Asana | Asana API, read | Events API sync token | Webhooks per resource, URL per webhook |
| GitHub | GitHub App, read on issues, pulls, contents | `since` and ETags | One webhook URL per GitHub App; repo webhooks by API |
| GitLab | OAuth app, `read_api` | `updated_after` | Project webhooks by API, URL per webhook |

### CRM and support

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| HubSpot | CRM API, `crm.objects.*.read` | Search by `hs_lastmodifieddate` | One webhook URL per app |
| Salesforce | REST, `api`, `refresh_token` | `SystemModstamp`, `getUpdated` | Change Data Capture over the Pub/Sub API (pull) |
| Pipedrive | Pipedrive API, read | `recents` since a timestamp | Webhooks by API, URL per webhook |
| Zendesk | Zendesk API, `read` | Incremental export cursor | Webhooks by API, URL per webhook |
| Intercom | Intercom API, read | Search by `updated_at` | One webhook URL per app |

### Finance and meetings

| Connector | API and scopes | Change feed | Push |
|---|---|---|---|
| Stripe | Restricted API key, read | `events` list (30 days), then lists by `created` | Webhook endpoints by API, URL per endpoint |
| Xero | Accounting API, `accounting.transactions.read`, `accounting.contacts.read` | `If-Modified-Since` | One webhook URL per app |
| QuickBooks Online | Accounting API, `com.intuit.quickbooks.accounting` | Change Data Capture endpoint (30 days) | One webhook URL per app |
| Exact Online | Exact REST, read | Sync API timestamps | Webhook subscriptions, URL per subscription |
| Zoom | Zoom API, `recording:read`, `meeting:read` | Lists by date | One event URL per app |

## What we need to get approved

### Once, for every provider

- **Mulholland as the publisher**: the legal entity, a verified domain, and one public homepage that names the app, says what it does with each provider's data, and links:
  - a privacy policy covering what is read, where it is stored (on the customer's own server when self-hosted), retention and deletion, with Google's Limited Use statement;
  - terms of service;
  - a support address and a security contact.
- **Branding**: one app name and logo used identically everywhere, matching the homepage. Changing either later re-triggers some reviews.
- **Demo videos** of the consent flow and how each scope is used, for Google and most marketplaces.
- **Security answers**: a security questionnaire, a vulnerability disclosure process, and for some marketplaces SOC 2 or a recent penetration test.

### Per provider

| Provider | What to register | Approval | Limits until approved |
|---|---|---|---|
| Google (Gmail, Drive, Calendar, Chat) | One Cloud project, OAuth consent screen (external), Desktop client; a Pub/Sub topic for Gmail push later | Brand verification; sensitive-scope verification for Calendar; restricted-scope verification for Gmail and Drive plus a CASA security assessment by an approved lab, repeated yearly | 100 users, behind an unverified-app warning |
| Microsoft (Outlook, SharePoint, OneDrive, Teams) | One multi-tenant Entra app, public client with device code | Publisher verification through the Microsoft AI Cloud Partner Program; Microsoft 365 App Compliance (Publisher Attestation, then Certification) helps admins approve | Many tenants block user consent to unverified apps; Teams scopes always need admin consent |
| Slack | One app, distributed, for hosted Archive; a manifest for self-hosters' internal apps | Slack Marketplace review | Non-Marketplace distributed apps get `conversations.history` and `conversations.replies` at one request a minute, 15 messages each; internal apps keep full limits |
| Dropbox | One app, scoped, full Dropbox read | Production approval, free, reviewed once 50 users link | 500 users; at 50 users, two weeks to apply before new links freeze; 5 teams for team apps |
| Box | One OAuth 2.0 app | Box Integrations listing review | Enterprise admins may need to enable it |
| Atlassian (Jira, Confluence) | One OAuth 2.0 (3LO) app with distribution on | Personal data declaration; apps that store personal data must use the Personal Data Reporting API; Marketplace security requirements if listed | None documented for distribution |
| Notion | One public integration | Company details, privacy and terms; Marketplace review if listed | None documented |
| HubSpot | One app on the Developer Platform | Marketplace listing review | 25 installs |
| Salesforce | One External Client App | AppExchange security review only to list; each customer org installs the app, or grants Approve Uninstalled Connected Apps, since September 2025 | Users without that permission cannot connect |
| Zendesk | One global OAuth client | Zendesk approval for a global client | Without it, a client per customer subdomain |
| Intercom | One public app | Intercom app review | Test workspaces only |
| Xero | One app | App Partner certification | 25 connections |
| QuickBooks Online | One app | Production keys after the app assessment questionnaire; Intuit App Partner Program terms and API fees | Sandbox only |
| Exact Online | One app in the App Center | App partner agreement for connecting other companies (to confirm) | To confirm |
| Zoom | One OAuth app | Marketplace publication review | Own account only |
| GitHub, GitLab, Linear, Asana, Pipedrive | One app each | None to use; listing in their directories is optional | None |
| Stripe | Nothing: restricted API keys | None | None |

### Token connectors, with no app and no approval

Where a service issues API tokens, a self-hoster can connect it today by pasting one: Stripe restricted keys, Notion internal integration tokens, Linear and Asana personal keys, GitHub fine-grained tokens, GitLab tokens (also the only way for self-managed GitLab), Atlassian API tokens, HubSpot private app tokens, Zendesk and Pipedrive API tokens, Intercom access tokens, and Slack internal apps from the manifest. Google, Microsoft, Dropbox, Box, Salesforce, Xero, QuickBooks and Zoom have no such route for other people's accounts, so they wait on the approvals above.

### Order

1. Homepage, privacy policy, terms and domain verification, since every review checks them.
2. Microsoft publisher verification, which is quick and unblocks Outlook, SharePoint and OneDrive.
3. Google brand and scope verification, and the CASA assessment, started early because it is the slowest. Development runs within the 100-user cap meanwhile.
4. Dropbox production, HubSpot, Slack and Xero listings once each has the installs it requires.
5. The rest as customers ask for them.

## Open questions

- **CASA for self-hosted instances**: Google's assessment covers apps that store or move restricted data through a server. A self-hosted Archive is a server, but the customer's, not Mulholland's. Ask Google whether the assessment covers Mulholland's hosted Archive only, or the shipped client as well.
- **Gmail push**: Pub/Sub pull from a Mulholland topic would need credentials on every instance. Polling `history.list` is cheap enough to start with.
- **A relay for one-URL providers**, for hosted Archive and self-hosters who opt in.

## Not doing

- Writing back to any provider.
- IMAP, folder watching and per-user scripts (Apps Script, Power Automate) as ways around OAuth.
- Third-party integration platforms (Nylas, Composio, Merge, Nango Cloud), which would route customers' data through another company.
