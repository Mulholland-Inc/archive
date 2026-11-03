<script>
  import Facts from '#lib/Facts.svelte';
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Section from '#lib/Section.svelte';

  const PROCESSORS = [
    ['Google Cloud', 'Runs the service in us-central1, in the United States: Cloud Run, a Cloud SQL Postgres database for each workspace, Cloud Storage for files, Secret Manager for keys and Cloud Tasks for scheduled work.'],
    ['Cloudflare', 'DNS, TLS and the proxy in front of api.archive.dev, and serves the web app at app.archive.dev.'],
    ['Anthropic', 'The model behind the in-app agent, when a workspace uses it. Anthropic does not train its models on what the service sends it.'],
    ['Google, Microsoft, Slack and GitHub', 'Sign-in, only with the provider you choose.'],
    ['The services you connect', 'Google, Microsoft, Intuit and the others in Accounts you connect send Archive what you allow it to read, under your own account with them.'],
    ['GitHub', 'Hosts archive.dev and the public code and issue tracker.'],
    ['Google Fonts', 'Serves the fonts on archive.dev.']
  ];
</script>

<Meta title="Privacy Policy — Archive" description="What Mulholland collects when you use Archive at app.archive.dev and archive.dev, how it is used, who else handles it, and how to have it deleted." />

<Hero kind="blog" title="Privacy Policy" lead="What we collect when you use Archive, what we do with it, and who else handles it. Effective November 3, 2026." />

<Section>
  <article class="prose">

<h2 id="scope">Who we are</h2>
<p>Mulholland Inc. ("Mulholland", "we") is a US company. We run Archive as a hosted service at app.archive.dev and api.archive.dev (the "service"), and this site at archive.dev. This policy covers both.</p>
<p>Archive is open-source software that anyone can run. If you use an Archive that you or someone else runs, this policy does not apply to it; whoever runs it decides how it handles your information.</p>
<p>A workspace belongs to the customer it was made for, usually an organization. Its owners decide who is invited, what goes in and when it is deleted. We handle what is in a workspace on the customer's behalf.</p>

<h2 id="collect">What we collect</h2>
<h3 id="collect-account">Your account</h3>
<ul>
<li><strong>Your invite</strong>: the email address a workspace owner invited you under.</li>
<li><strong>Sign-in</strong>: when you sign in with Google, Microsoft, Slack or GitHub, we ask the provider for your email address and basic profile, and keep your email address, your name and the provider's id for your account. We use whether the provider has verified your email address to match you to your invite.</li>
<li><strong>Passkeys</strong>: the public key of each passkey you add. The private key stays on your device.</li>
<li><strong>Sessions and tokens</strong>: the session that keeps you signed in, and the API and agent tokens you make. We keep only a hash of each.</li>
</ul>
<h3 id="collect-content">Your content</h3>
<p>Everything you and your team put in a workspace: records, files, views, workflows and their runs, chats with the in-app agent and secret fields. It is your content, and we use it only to run the service for you.</p>
<h3 id="collect-connected">Accounts you connect</h3>
<p>You can connect accounts you hold at other services so that Archive can read them for you and your workspace. Each person connects their own mail, calendar and files, and only an owner connects an organization's accounts, like its accounting, for the workspace. Archive asks each service for read-only access wherever it offers it, and reads only what you allow on that service's own consent screen.</p>
<ul>
<li><strong>Google</strong>: with your consent, Gmail messages and attachments (read only), Google Calendar events and attendees (read only), and Google Drive files and their contents (read only), with your email address to name the account.</li>
<li><strong>Microsoft</strong>: with your consent, Outlook mail and attachments, your Outlook calendar, the files of the SharePoint sites and OneDrive you can open, and your Microsoft Teams chats and the posts of your teams' channels, with the email addresses of who sent them, all read only, with your email address to name the account.</li>
<li><strong>Intuit QuickBooks</strong>: with an owner's consent, one company's books: its chart of accounts, customers, vendors, employees, products and services, and transactions such as invoices, bills, payments and journal entries. Intuit offers no read-only access, so Archive asks for its accounting access and only reads with it.</li>
<li><strong>Salesforce, HubSpot and Klaviyo</strong>: with an owner's consent, the organization's companies or accounts, contacts, leads, deals and opportunities, marketing profiles and campaigns, read only.</li>
<li><strong>Karbon, Shopify and Meta Ads</strong>: with keys an owner makes in that service and pastes into Archive, the firm's clients, people and work items, the store's orders, products and customers, or the ad account's campaigns and daily results, read only. The keys are kept encrypted like tokens, and end when they are revoked there.</li>
<li><strong>Open Dental</strong>: with a customer key a dental practice makes for Archive, its providers, patients, appointments, procedures and payments, read only. This is protected health information: we handle it under the practice's business associate agreement with Mulholland, only to run the service for the practice, and it is seen only by the people the practice's owners allow.</li>
<li><strong>Others</strong>, like Notion, Gusto, GitHub, Asana, Linear, Sentry and Stripe, are read and acted on only as you allow when you connect them.</li>
</ul>
<p>For each connected account, Archive keeps the refresh token the service gives it, encrypted with the workspace's own key, and a copy of what it reads in the workspace's database: for example the text, people and dates of a message or an event, the name, details and words of a document, and the records of a company's books. It uses that copy to let you and the people and tools your workspace allows search, read and work with it, and to keep it current when the service says something changed.</p>
<p>What a person connects for themselves is seen only by them and the tools and agents acting as them. What an owner connects for the workspace is seen by the people the owner allows.</p>

<h3 id="google-limited-use">Google user data</h3>
<p>Archive's use and transfer to any other app of information received from Google APIs will adhere to the <a href="https://developers.google.com/terms/api-services-user-data-policy">Google API Services User Data Policy</a>, including the Limited Use requirements. In particular, we:</p>
<ul>
<li>use Google user data only to provide and improve the features you use it for in Archive: searching, reading and working with your own mail, calendar and files;</li>
<li>transfer it to others only as needed to provide those features, such as to Anthropic when you ask the in-app agent about it, to comply with the law, or as part of a merger or sale with notice to you;</li>
<li>never use it for advertising, never sell it, and never use it to develop, improve or train generalized AI or machine-learning models;</li>
<li>never let a person read it unless you ask us to and agree, it is needed for security, such as investigating abuse, it is needed to comply with the law, or it is aggregated and anonymized for operations.</li>
</ul>
<p>We hold data from Microsoft, Intuit and every other service you connect to the same standard.</p>

<h3 id="collect-requests">Requests</h3>
<p>Google Cloud and Cloudflare record requests to the service, such as your IP address, browser, the time and the address requested, so we can run it, fix problems and protect it.</p>
<h3 id="collect-site">This site</h3>
<p>archive.dev has no analytics, no tracking, no ads and no cookies. GitHub serves its pages and Google Fonts serves its fonts, so both see your IP address and browser, as any web server does. Your browser keeps your theme and whether you closed the explainer in its local storage, and they never leave your device.</p>

<h2 id="cookies">Cookies and browser storage</h2>
<p>The service sets one session cookie on api.archive.dev when you sign in. It lasts up to 30 days and is only sent over HTTPS. Short-lived cookies hold a sign-in or passkey step while it is in progress. There are no advertising or tracking cookies.</p>
<p>The web app keeps your current workspace, and which passkeys this device has, in your browser's local storage.</p>

<h2 id="use">How we use it</h2>
<ul>
<li>To sign you in and keep you signed in.</li>
<li>To store, search and serve your content to the people and tokens your workspace allows.</li>
<li>To run the in-app agent and your workflows when you use them.</li>
<li>To keep the service secure, fix problems and answer you when you write to us.</li>
</ul>
<p>We do not sell personal information or customer content, and we do not use either for advertising. We share them only with the providers below, as needed to run the service, or when the law requires it.</p>

<h2 id="agent">The in-app agent</h2>
<p>Archive calls an AI model only when someone in a workspace chats with its agent. The service then sends the conversation to Anthropic's API: the messages, the name and email address of the person chatting, their role and groups, a description of the workspace's tables, and the results of the code the agent runs, which can include any records and file contents that person can see, among them what their connected accounts synced.</p>
<p>Anthropic processes it to answer and does not use it to train its models. Neither do we: we do not train AI models on your content or on what your connected accounts hold.</p>
<p>The service always uses Mulholland's Anthropic account. Chats are kept in the workspace until they are deleted.</p>
<p>When you connect your own AI app over MCP or the API, or write a workflow that calls another service, what it reads goes to that provider under your own account with them.</p>

<h2 id="processors">Who else handles it</h2>
<p>These providers process information for us to run the service and this site.</p>
<Facts head={['Provider', 'What they do']} rows={PROCESSORS} />
<p>If you report a bug or ask a question on GitHub, what you post there is public.</p>

<h2 id="where">Where it is stored</h2>
<p>Your account and content are stored in Google Cloud's us-central1 region, in the United States. Requests pass through Cloudflare's network nearest to you.</p>

<h2 id="security">Security</h2>
<ul>
<li>Each workspace has its own Postgres database and its own database login, which can reach only that database.</li>
<li>Postgres roles and row-level security check every request, so each person, token and agent sees only what their role and groups allow.</li>
<li>Secret fields, and the tokens of the accounts you connect, are encrypted with the workspace's own key, which is itself encrypted with a key kept in Google Secret Manager.</li>
<li>Databases, backups and files are encrypted at rest by Google Cloud.</li>
<li>Traffic is encrypted with TLS, from your browser to Cloudflare and from Cloudflare to Google.</li>
<li>Anyone with a file's share link can open that file until the link expires.</li>
</ul>
<p>No system is perfectly secure. If you find a problem, write to <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a>.</p>

<h2 id="retention">Keeping and deleting</h2>
<ul>
<li>Your content stays until you or your workspace's owners delete it, or the workspace is deleted.</li>
<li>Removing someone from a workspace deletes their access, tokens, chats and workflows there. The records and files they added stay.</li>
<li>Deleting a workspace drops its database, with every record, chat, workflow and secret field in it. To have its files removed from storage too, write to us.</li>
<li>Google Cloud keeps automatic database backups for a limited time, so deleted data can remain in a backup until it expires.</li>
<li>Disconnecting an account deletes everything Archive synced from it, and revokes Archive's access at the service where the service allows it. You can also remove Archive's access at the service itself, like your Google Account's third-party access page, and Archive then stops reading it.</li>
<li>Sessions end after 30 days. Tokens end when they expire or are deleted.</li>
<li>Your account stays while you belong to a workspace. Write to us to have it deleted.</li>
</ul>

<h2 id="rights">Your choices and rights</h2>
<p>You can read, change, export and delete your content in the app, the API and the CLI. You can also ask us for a copy of the personal information we hold about you, to correct it or to delete it. For content in an organization's workspace, we may ask its owners to act on your request. We will not treat you differently for asking.</p>

<h2 id="children">Children</h2>
<p>The service is not meant for children under 13, and we do not knowingly collect their information.</p>

<h2 id="changes">Changes</h2>
<p>When we change this policy, we post the new version here with a new effective date. If a change matters, we tell workspace owners before it takes effect.</p>

<h2 id="contact">Contact</h2>
<p>Mulholland Inc., <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a>.</p>
  </article>
</Section>
