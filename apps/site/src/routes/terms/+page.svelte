<script>
  import Hero from '#lib/Hero.svelte';
  import Meta from '#lib/Meta.svelte';
  import Section from '#lib/Section.svelte';
  import { GITHUB } from '#lib/site.js';
</script>

<Meta title="Terms of Service — Archive" description="The terms for using Archive as hosted by Mulholland at app.archive.dev and archive.dev: accounts, your content, acceptable use, availability and liability." />

<Hero kind="blog" title="Terms of Service" lead="The terms for using Archive as hosted by Mulholland. Effective October 30, 2026." />

<Section>
  <article class="prose">

<h2 id="agreement">These terms</h2>
<p>These terms are an agreement between you and Mulholland Inc. ("Mulholland", "we") for using Archive as we host it at app.archive.dev and api.archive.dev (the "service"), and this site at archive.dev. By using them, you accept these terms. If you use the service for an organization, you accept them on its behalf and confirm you may.</p>
<p>If your organization has a separate written agreement with Mulholland, that agreement wins where the two differ. The <a href="/privacy">Privacy Policy</a> explains how we handle personal information.</p>

<h2 id="open-source">Open source</h2>
<p>Archive's source code is public on <a href={GITHUB}>GitHub</a>, and anyone can run it themselves. These terms cover only the service Mulholland runs. They do not grant or limit any rights in the source code, which come from its repository.</p>

<h2 id="accounts">Accounts and invites</h2>
<ul>
<li>The service is invite-only. Mulholland makes each workspace for a customer and names its first owner, and the workspace's owners invite everyone else by email.</li>
<li>You sign in with Google, Microsoft, Slack or GitHub under the email address you were invited with, or with a passkey.</li>
<li>Keep your sign-in, passkeys and tokens safe. You are responsible for what is done with them, including by the agents, apps and workflows you connect or make. Tell us right away if you think one has been misused.</li>
<li>Owners are responsible for who they invite and the access they give.</li>
</ul>

<h2 id="content">Your content</h2>
<p>You own what you put in the service, and so does the customer whose workspace it is in. We claim no ownership of it.</p>
<p>You let Mulholland store, copy, process and send your content only as needed to run the service for you, including through the providers listed in the Privacy Policy, such as Anthropic when your workspace uses the in-app agent. We do not sell your content or use it for advertising.</p>
<p>You confirm that you have the right to put your content in the service, and that doing so breaks no law and no one else's rights. You can export your content at any time through the app, the API and the CLI.</p>

<h2 id="use">Acceptable use</h2>
<p>Do not use the service to:</p>
<ul>
<li>break the law, or infringe or violate anyone's rights;</li>
<li>store or share content you have no right to, or that is illegal;</li>
<li>upload malware, send spam, or attack or overload other systems, including from scripts and workflows;</li>
<li>reach another workspace's data, or get around access rules, limits or sign-in;</li>
<li>disrupt the service, or test its security without our permission;</li>
<li>resell the service without a written agreement with us.</li>
</ul>
<p>We may remove content or suspend access that breaks these rules.</p>

<h2 id="agents">AI agents</h2>
<p>The in-app agent, and any agent you connect, acts with the access of the person or token it uses. Agents can make mistakes, so check their work before you rely on it. What an agent writes for you is your content.</p>

<h2 id="availability">Availability</h2>
<p>We work to keep the service running, but it may be slow or unavailable at times, including for maintenance. We may change, add or remove features. If we stop offering the service, we will tell workspace owners ahead of time and give them a chance to export their content.</p>

<h2 id="warranty">No warranty</h2>
<p>The service is provided "as is" and "as available", without warranties of any kind, express or implied, including merchantability, fitness for a particular purpose and non-infringement, to the fullest extent the law allows. We do not promise that the service will be uninterrupted or error-free, or that content will never be lost. Keep your own copies of what matters.</p>

<h2 id="liability">Limitation of liability</h2>
<p>To the fullest extent the law allows, Mulholland is not liable for any indirect, incidental, special, consequential or punitive damages, or for lost profits, revenue, data or goodwill, arising from the service or these terms. Our total liability for any claim is limited to the greater of what you paid us for the service in the 12 months before the claim, or $100.</p>

<h2 id="termination">Termination</h2>
<p>You can stop using the service at any time, and a workspace owner can ask us to delete their workspace. Owners can remove people from their workspace.</p>
<p>We may suspend or end your access if you break these terms, if we must to protect the service or others, or if the law requires it. Where we reasonably can, we tell you first and give you a chance to export your content. After that, content is deleted as the Privacy Policy describes. The sections on your content, no warranty, limitation of liability and general terms still apply after access ends.</p>

<h2 id="general">General</h2>
<p>These terms are governed by the laws of the United States and of the state where Mulholland Inc. is incorporated, without regard to conflict of law rules. If part of these terms cannot be enforced, the rest still applies. Not enforcing a term is not a waiver of it. You may not transfer these terms without our consent. These terms and the Privacy Policy are the whole agreement about the service, unless you have a separate written agreement with us.</p>

<h2 id="changes">Changes</h2>
<p>When we change these terms, we post the new version here with a new effective date. If a change matters, we tell workspace owners before it takes effect. Using the service after that means you accept the new terms.</p>

<h2 id="contact">Contact</h2>
<p>Mulholland Inc., <a href="mailto:hello@mulholland.ai">hello@mulholland.ai</a>.</p>
  </article>
</Section>
