# Help

Carries the app's Help chat to a private Discord forum and brings the replies back. It stands apart from the engine.

## Set it up

1. Make a private Discord server with a forum channel, say #archive-help.
2. In the forum's settings, under Integrations, create a webhook and copy its URL.
3. At discord.com/developers, create an application and a bot, turn on Message Content Intent, and invite it to the server with permission to read the forum.
4. Deploy, setting the secrets:

```sh
npx wrangler secret put DISCORD_WEBHOOK
npx wrangler secret put DISCORD_BOT_TOKEN
npx wrangler secret put HELP_SECRET
npx wrangler deploy
```

`HELP_SECRET` is any long random string. Add the app's address to `ORIGINS`, and build the app with `VITE_HELP_URL` set to the worker's address. Without it, the chat stays a demo.

`VITE_ARCHIVIST` sets the archivist's name, and `VITE_BOOK_URL` turns on Book a call with a scheduling link.

## Limits

- The name, email and archive on a message come from the browser, so treat them as unverified.
- Add a rate limit rule in Cloudflare before this faces the public: `ORIGINS` stops other sites, not scripts.

Run the tests with `node --test`.
