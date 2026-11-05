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
    ['The services you connect', 'Google, Microsoft, Intuit, Oracle, YNAB and the others in Accounts you connect send Archive what you allow it to read, under your own account with them.'],
    ['Plaid', 'Connects the bank, brokerage and loan accounts an owner picks, in its own window, and passes Archive what the owner allows.'],
    ['NexHealth', 'Reads the Dentrix of a dental practice that connects it, for Archive.'],
    ['GitHub', 'Hosts archive.dev and the public code and issue tracker.'],
    ['Google Fonts', 'Serves the fonts on archive.dev.']
  ];
</script>

<Meta title="Privacy Policy — Archive" description="What Mulholland collects when you use Archive at app.archive.dev and archive.dev, how it is used, who else handles it, and how to have it deleted." />

<Hero kind="blog" title="Privacy Policy" lead="What we collect when you use Archive, what we do with it, and who else handles it. Last updated and effective November 5, 2026." />

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
<p>Everything you and your team put in a workspace: records, files, views, workflows and their runs, chats with the in-app agent and secret fields. It is your content, and we use it only to run the service for you, and, where your organization's agreement with Mulholland allows it, to make de-identified data, as <a href="#models">Models we make</a> describes.</p>
<h3 id="collect-connected">Accounts you connect</h3>
<p>You can connect accounts you hold at other services so that Archive can read them for you and your workspace. Each person connects their own mail, calendar and files, and their own personal accounts, like their health, money and music, and only an owner connects an organization's accounts, like its accounting or its bank, for the workspace. Archive only reads to keep its copy current. It changes something at a service only when you, or a tool or agent acting as you, asks it to, like sending an email from your Gmail. It asks each service for no more access than that needs, and uses only what you allow on that service's own consent screen.</p>
<ul>
<li><strong>Google</strong>: with your consent, Gmail messages and attachments, to read them and, when you ask, to send, draft and label mail; Google Calendar events and attendees, to read them and, when you ask, to make, change and cancel events; and Google Drive files and their contents, to read them and, when you ask, to make, change and trash files; with your email address to name the account. Archive never deletes mail or files for good.</li>
<li><strong>Google Nest</strong>: with your consent, the homes, rooms and devices you share with Archive in Google's Nest permissions: each thermostat, camera, doorbell and display, whether it is online, and a thermostat's mode, temperature and setpoints, to read them and, when you ask, to change a thermostat's mode and temperature.</li>
<li><strong>Microsoft</strong>: with your consent, Outlook mail and attachments, your Outlook calendar, and your Microsoft Teams chats and the posts of your teams' channels, with the email addresses of who sent them, all read only; and the files of the SharePoint sites and the OneDrive, work or personal, that you can open, to read them and, when you ask, to add and replace files; with your email address to name the account.</li>
<li><strong>Microsoft Dynamics 365 Business Central</strong>: with an owner's consent, or with a web service access key for a user an owner sets up on the organization's own Business Central server, the customers, vendors, items, sales quotes, orders and invoices, purchase orders and invoices, chart of accounts and ledger entries of its companies, to read them and, when you ask, to add and change sales quotes and customers. Archive never posts or deletes anything in Business Central.</li>
<li><strong>Intuit QuickBooks</strong>: with an owner's consent, one company's books: its chart of accounts, customers, vendors, employees, products and services, and transactions such as invoices, bills, payments and journal entries. Intuit offers no read-only access, so Archive asks for its accounting access and only reads with it.</li>
<li><strong>Oracle NetSuite</strong>: with an owner's consent, one NetSuite account's books, as far as the NetSuite role they sign in with may see: its chart of accounts, customers, vendors, employees, items, and transactions such as invoices, bills, payments and journal entries, with their lines, read only, with the company's name or NetSuite account ID to name the account.</li>
<li><strong>Zoho Books</strong>: with an owner's consent, the books of the organization's Zoho Books organizations: chart of accounts, contacts, invoices, bills, customer and vendor payments and journals, read only.</li>
<li><strong>Ramp and Brex</strong>: with an owner's consent, the company's card transactions and expenses, cards, people, departments, reimbursements, receipts, bills, budgets and vendors, read only.</li>
<li><strong>HubSpot</strong>: with an owner's consent, the organization's contacts, companies and deals, to read them and, when you ask, to add, change, link and archive them. An archived record stays in HubSpot's recycling bin, where it can be restored.</li>
<li><strong>Zoho CRM</strong>: with an owner's consent, the organization's leads, contacts, accounts and deals, to read them and, when you ask, to add and change them. Archive never deletes them.</li>
<li><strong>Salesforce and Klaviyo</strong>: with an owner's consent, the organization's accounts, contacts, leads and opportunities, or its marketing profiles and campaigns, read only.</li>
<li><strong>Mailchimp</strong>: with an owner's consent, the organization's audiences, their members and tags, and its campaigns and their reports, to read them and, when you ask, to add, update and tag members.</li>
<li><strong>Shopify</strong>: with an owner's consent when they add Archive's app to the store, or with the token of a custom app the store made before 2026, the store's orders, products, customers and inventory, to read them and, when you ask, to change products, prices, stock, draft orders, orders' notes and tags, and customers.</li>
<li><strong>ClickUp</strong>: with an owner's consent, the workspaces they pick, with their spaces, lists, tasks and comments, to read them and, when you ask, to add and change tasks and post comments.</li>
<li><strong>Karbon and Meta Ads</strong>: with keys an owner makes in that service and pastes into Archive, the firm's clients, people and work items, or the ad account's campaigns and daily results, read only.</li>
<li><strong>Vanta</strong>: with an app an owner makes in Vanta and pastes into Archive, the organization's frameworks, controls, tests, vendors, people, policies and documents, read only.</li>
<li><strong>Snowflake and Databricks</strong>: with the key or secret of a service user an owner makes for Archive, the details of the tables the owner picks, and the SQL that you, or a tool or agent acting as you, ask Archive to run, as far as that user's grants allow. Archive changes data there only when asked and only where the owner granted that user write access.</li>
<li><strong>Open Dental and Dentrix</strong>: with a customer key a dental practice makes for Archive in Open Dental, or, for Dentrix, through NexHealth's Synchronizer, which the practice installs and which NexHealth runs for Archive, its providers, patients, appointments, procedures and payments, read only. This is protected health information: we handle it under the practice's business associate agreement with Mulholland, only to run the service for the practice, and it is seen only by the people the practice's owners allow.</li>
<li><strong>Banks, brokerages and lenders, through Plaid</strong>: with an owner's consent, given in Plaid's own window where they sign in to their institution, the accounts of one login, their balances and transactions and, where the institution has them, investment holdings and trades, and the terms of credit cards, student loans and mortgages, read only. Plaid handles the sign-in and passes Archive what the owner allows, under <a href="https://plaid.com/legal/#end-user-privacy-policy">Plaid's End User Privacy Policy</a>. Archive never receives the password for the institution, and never trades or moves money.</li>
<li><strong>YNAB</strong>: with your consent, given when you sign in to YNAB and allow Archive read-only access, or with a personal access token you make in YNAB and paste into Archive, your plans' accounts, transactions and categories, read only. See <a href="#ynab">YNAB data</a>.</li>
<li><strong>Coinbase</strong>: with your consent, your wallets, their balances and every send, receive, buy and sell, read only. Archive asks Coinbase only for read access, and cannot buy, sell, send or withdraw.</li>
<li><strong>Gemini</strong>: with a read-only Auditor API key you make in Gemini and paste into Archive, your balances, staking, trades, deposits and withdrawals. An Auditor key cannot trade or withdraw.</li>
<li><strong>Oura, WHOOP and Fitbit</strong>: with your consent, your sleep, readiness or recovery, activity and strain, workouts, heart rate, heart rate variability, blood oxygen, breathing rate and weight, as each service measures them, with your email address or profile to name the account, read only. Fitbit and Pixel Watch data comes through Google's Health API.</li>
<li><strong>MyChart and Oracle Health</strong>: with your consent, given at your health system's own sign-in, your medical record as that system shares it with apps you choose: your conditions, medications, allergies, lab results, vital signs, immunizations, visits, reports and documents, read only, for as long as you allow it there.</li>
<li><strong>The Archive iPhone app</strong>: with your permission on the phone, your Apple Health samples and workouts, the details of your Photos library, such as each photo's name, dates, size, place and whether it is a favorite, with a small thumbnail of each, your contacts and your calendar events, read only. You can change each permission in your iPhone's Settings.</li>
<li><strong>iCloud</strong>: with your Apple ID and an app-specific password you make for Archive, never your Apple ID password, your iCloud Mail and its attachments, your calendars and your contacts, read only.</li>
<li><strong>Dropbox</strong>: with your consent, your files and folders and their contents, to read them and, when you ask, to add and replace files.</li>
<li><strong>Evernote</strong>: with your consent, given when you sign in to Evernote and allow Archive, your account's email and username, your notebooks, your notes and their tags, and the files in your notes, read only. Archive never changes, adds or deletes anything in Evernote. Disconnecting Evernote in Archive ends Archive's access there too.</li>
<li><strong>Spotify</strong>: with your consent, your profile, the tracks you saved, your playlists, what you played lately and play most, and the artists you follow, read only.</li>
<li><strong>Tesla</strong>: with your consent, your profile and vehicles, each vehicle's battery, range, mileage and software when it is awake, and your Supercharging history and invoices, read only. Archive never sends your vehicles commands.</li>
<li><strong>UPS and FedEx</strong>: with API keys you make at UPS's or FedEx's developer site, Archive finds tracking numbers in the mail you synced to Archive, and any you ask about, and sends only those numbers to UPS or FedEx to look up where each package is.</li>
<li><strong>ClassDojo, ParentSquare and PowerSchool</strong>: Archive does not connect to these services. It finds their notices in the mail you already synced from Gmail or Outlook, and matches each to a child by the first names you give it. Nothing is sent to them.</li>
<li><strong>Others</strong>, like Notion, Gusto, GitHub, Asana, Linear, Sentry and Stripe, are read and acted on only as you allow when you connect them.</li>
</ul>
<p>For each connected account, Archive keeps the refresh token the service gives it, or the key, token or password you paste, encrypted with the workspace's own key, and a copy of what it reads in the workspace's database: for example the text, people and dates of a message or an event, the name, details and words of a document, and the records of a company's books. It uses that copy to let you and the people and tools your workspace allows search, read and work with it, and to keep it current when the service says something changed.</p>
<p>What a person connects for themselves is seen only by them and the tools and agents acting as them. What an owner connects for the workspace is seen by the people the owner allows.</p>
<p>Disconnecting an account deletes its token or key and everything Archive synced from it. Archive also revokes its own access at Google, Salesforce, Intuit, NetSuite, Zoho, HubSpot, Klaviyo, Shopify, Ramp, Brex, Plaid, Coinbase, Oura, WHOOP, Dropbox, Asana and GitHub. At any other service, remove Archive in that service's own settings, and delete any key, token or app-specific password you made for Archive there.</p>

<h3 id="google-limited-use">Google user data</h3>
<p>Archive's use and transfer to any other app of information received from Google APIs will adhere to the <a href="https://developers.google.com/terms/api-services-user-data-policy">Google API Services User Data Policy</a>, including the Limited Use requirements. In particular, we:</p>
<ul>
<li>use Google user data only to provide and improve the features you use it for in Archive: searching, reading and working with your own mail, calendar and files, and sending mail, saving files and scheduling events when you ask;</li>
<li>transfer it to others only as needed to provide those features, such as to Anthropic when you ask the in-app agent about it, to comply with the law, or as part of a merger or sale with notice to you;</li>
<li>never use it for advertising, never sell it, and never use it to develop, improve or train generalized AI or machine-learning models;</li>
<li>never let a person read it unless you ask us to and agree, it is needed for security, such as investigating abuse, it is needed to comply with the law, or it is aggregated and anonymized for operations.</li>
</ul>
<p>We hold data from Microsoft, Intuit, Oracle and every other service you connect to the same standard, with one difference: where a service's terms allow it and your organization's agreement with Mulholland does too, we may de-identify that data as <a href="#models">Models we make</a> describes. We never do that with Google user data.</p>

<h3 id="ynab">YNAB data</h3>
<p>When you connect YNAB, this is how Archive handles what it gets from the YNAB API.</p>
<ul>
<li><strong>How you connect</strong>: either you sign in to YNAB and allow Archive read-only access, or you make a personal access token in YNAB's Developer Settings and paste it into Archive. Archive never asks for your YNAB password or for the sign-in of any bank you link to YNAB.</li>
<li><strong>What Archive reads</strong>: your YNAB user ID, and for each of your plans its name and currency, its accounts and their balances, its transactions with their dates, amounts, payees, categories, memos and whether they have cleared, and this month's categories with what each has assigned, spent and left, and its goal.</li>
<li><strong>Read only</strong>: Archive asks YNAB only for read-only access, and only reads YNAB's user and plan data. A personal access token could also change your plans, but Archive only reads with it. Archive never changes your plans or moves money.</li>
<li><strong>Why</strong>: only so that you, and the tools and agents acting as you, can see, search and work with your own plans in Archive, and to keep Archive's copy current.</li>
<li><strong>Who sees it</strong>: YNAB is a personal connection. What it syncs is seen only by you and the tools and agents acting as you, not by the rest of your workspace.</li>
<li><strong>How it is stored and secured</strong>: Archive keeps your YNAB token encrypted with your workspace's own key, which is itself encrypted with a key kept in Google Secret Manager. The copy of your plans is kept in your workspace's own Postgres database in Google Cloud's us-central1 region, in the United States, encrypted at rest, and reached only over TLS. Postgres row-level security keeps it to you.</li>
<li><strong>Who else gets it</strong>: data from the YNAB API is never passed to a third party without your knowledge. Google Cloud stores it and Cloudflare carries it, to run the service. It goes to Anthropic only when you ask the in-app agent about it, to an AI app or tool only when you connect that app to Archive yourself, and to anyone else only when the law requires it. We never sell it, never use it for advertising, and never use it to train AI models.</li>
<li><strong>How long it is kept</strong>: while YNAB stays connected. Each sync updates the copy, and removes what you deleted in YNAB. Disconnecting YNAB in Archive deletes your token and everything synced from YNAB at once. Google Cloud's automatic database backups can hold it for up to seven more days.</li>
<li><strong>Removing access</strong>: you can also remove Archive's access in YNAB's account settings, or delete the personal access token you made in YNAB's Developer Settings, and Archive then stops reading.</li>
<li><strong>Deleting on request</strong>: to have us delete your YNAB data, disconnect YNAB in Archive, or write to <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a> and we will delete it.</li>
<li><strong>Changes</strong>: if Archive is going to read more of your YNAB data, or use it for something else, we will update this policy first and ask you to allow it again before Archive reads it.</li>
</ul>
<p class="small">We are not affiliated, associated, or in any way officially connected with YNAB or any of its subsidiaries or affiliates. The official YNAB website can be found at <a href="https://www.ynab.com">https://www.ynab.com</a>. The names YNAB and You Need A Budget, as well as related names, tradenames, marks, trademarks, emblems, and images are registered trademarks of YNAB.</p>

<h3 id="plaid">Plaid data</h3>
<p>When an owner connects a bank, brokerage or lender through Plaid, this is how Archive handles what it gets from Plaid.</p>
<ul>
<li><strong>How it connects</strong>: the owner picks their institution and signs in to it in Plaid's own window, and chooses there which accounts to share. Plaid handles that sign-in under <a href="https://plaid.com/legal/#end-user-privacy-policy">Plaid's End User Privacy Policy</a>. Archive never sees or keeps the password for the institution. Plaid gives Archive an access token for that one login.</li>
<li><strong>What Archive reads</strong>: the name of the institution, and for each shared account its name, type, the last few digits of its number and its balances; up to two years of transactions, pending ones too, with their dates, amounts, merchants, categories and descriptions; and, where the institution has them, investment holdings and trades, and the rates, payments and due dates of credit cards, student loans and mortgages. Archive does not ask Plaid for full account and routing numbers or for identity details.</li>
<li><strong>Read only</strong>: Archive never trades, pays, transfers or moves money through Plaid.</li>
<li><strong>Why</strong>: only so that the people and tools the owner allows can see, search and work with the organization's accounts in Archive, and to keep Archive's copy current when Plaid says something changed.</li>
<li><strong>Who sees it</strong>: Plaid is an organization connection. Only an owner connects it, and what it syncs is seen by the people the owner allows.</li>
<li><strong>How it is stored and secured</strong>: the Plaid access token is encrypted with the workspace's own key, which is itself encrypted with a key kept in Google Secret Manager. The copy of the accounts is kept in the workspace's own Postgres database in Google Cloud's us-central1 region, in the United States, encrypted at rest, and reached only over TLS. Postgres roles and row-level security limit it to the people the owner allows. Archive checks Plaid's signature on every message Plaid sends it.</li>
<li><strong>Who else gets it</strong>: Google Cloud stores it and Cloudflare carries it, to run the service. It goes to Anthropic only when someone asks the in-app agent about it, to an AI app or tool only when someone connects that app to Archive themselves, and to anyone else only when the law requires it. We never sell it, never use it for advertising, and never use it to train AI models.</li>
<li><strong>How long it is kept</strong>: while the login stays connected. Disconnecting it in Archive removes the login at Plaid, so Plaid stops sharing its data with Archive, and deletes the access token and everything synced through Plaid at once. Google Cloud's automatic database backups can hold it for up to seven more days.</li>
<li><strong>Deleting on request</strong>: disconnect the login in Archive, or write to <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a> and we will delete it. To manage what Plaid itself keeps, use <a href="https://my.plaid.com">Plaid Portal</a>.</li>
</ul>

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
<li>Where a customer's agreement with Mulholland allows it, to make de-identified data and train our own models with it, as <a href="#models">Models we make</a> describes.</li>
</ul>
<p>We do not sell personal information or customer content, and we do not use either for advertising. We share them only with the providers below, as needed to run the service, or when the law requires it.</p>

<h2 id="agent">The in-app agent</h2>
<p>Archive calls an AI model only when someone in a workspace chats with its agent. The service then sends the conversation to Anthropic's API: the messages, the name and email address of the person chatting, their role and groups, a description of the workspace's tables, and the results of the code the agent runs, which can include any records and file contents that person can see, among them what their connected accounts synced.</p>
<p>Anthropic processes it to answer and does not use it to train its models. We train our own models only on de-identified data, and only as <a href="#models">Models we make</a> describes.</p>
<p>The service always uses Mulholland's Anthropic account. Chats are kept in the workspace until they are deleted.</p>
<p>When you connect your own AI app over MCP or the API, or write a workflow that calls another service, what it reads goes to that provider under your own account with them.</p>

<h2 id="models">Models we make</h2>
<p>Mulholland makes decision models and fine-tuned language models. Where a workspace belongs to a customer whose <a href="https://mulholland.ai/legal">Master Services Agreement</a> with Mulholland is on version 1.4 or later, we may de-identify information in that workspace and use the de-identified data, alone or combined with de-identified data from other customers and sources, for any lawful purpose, including to develop and train our own models. We may sell, license or otherwise make those models available to others, on their own or as part of our products.</p>
<ul>
<li><strong>Which data</strong>: only information we first receive or generate on or after the date the customer first agrees to one of those versions, never earlier information or new information made by reprocessing it. Customers on earlier versions, and workspaces with no such agreement, are not affected. Archive stays open source, and an Archive that you or someone else runs sends Mulholland nothing, so none of its data is used.</li>
<li><strong>Never used</strong>: data from Google APIs, from YNAB and from Plaid, Apple Health data from the iPhone app, and data from any other connected service whose terms do not allow it. We also never de-identify information about children under 18, or tax return information.</li>
<li><strong>How</strong>: we de-identify inside the customer's own workspace before combining the result with anything else, and first remove credentials, access tokens, payment-card data, government ID numbers, financial-account numbers and biometric identifiers. Protected health information is de-identified only under HIPAA's expert determination method, or, for structured fields only, its safe harbor method.</li>
<li><strong>Limits</strong>: we keep and use de-identified data only in de-identified form, and will not try to re-identify it. We do not sell or license the de-identified data itself as a dataset. We never use identifiable customer data to train a model used for another customer, and never let Anthropic or any other outside model provider train its models on customer data or on de-identified data made from it.</li>
</ul>
<p>The customer's agreement governs, and Mulholland's <a href="https://mulholland.ai/policies/customer-data">Customer Data Privacy Notice</a> summarizes it.</p>

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
<li>Google Cloud keeps automatic database backups for up to seven days, so deleted data can remain in a backup until it expires.</li>
<li>Disconnecting an account deletes everything Archive synced from it, and revokes Archive's access at the service where the service allows it. You can also remove Archive's access at the service itself, like your Google Account's third-party access page, and Archive then stops reading it.</li>
<li>Sessions end after 30 days. Tokens end when they expire or are deleted.</li>
<li>Your account stays while you belong to a workspace. Write to us to have it deleted.</li>
</ul>

<h2 id="rights">Your choices and rights</h2>
<p>You can read, change, export and delete your content in the app and the API. You can also ask us for a copy of the personal information we hold about you, to correct it or to delete it. For content in an organization's workspace, we may ask its owners to act on your request. We will not treat you differently for asking.</p>

<h2 id="children">Children</h2>
<p>The service is not meant for children under 13, and we do not knowingly collect their information.</p>

<h2 id="changes">Changes</h2>
<p>When we change this policy, we post the new version here with a new effective date. If a change matters, we tell workspace owners before it takes effect.</p>

<h2 id="contact">Contact</h2>
<p>Mulholland Inc., <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a>.</p>
  </article>
</Section>
