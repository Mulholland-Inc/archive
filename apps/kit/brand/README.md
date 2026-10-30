# Archive brand

| File | Use it for |
|---|---|
| `icon-120.png` | Google Cloud: OAuth consent screen, App logo |
| `icon-215.png` | Microsoft Entra: App registration, Branding & properties, Logo |
| `icon-512.png` | Slack app icon, GitHub OAuth app logo |
| `icon-1024.png`, `icon.svg` | App stores and anywhere that asks for a large square icon |
| `icon-dark-1024.png`, `icon-dark.svg` | The square icon on a dark background |
| `mark.svg`, `mark.png` | The mark alone, on light backgrounds |
| `mark-white.svg`, `mark-white.png` | The mark alone, on dark backgrounds |
| `wordmark.svg`, `wordmark.png` | The logo with the name, on light backgrounds |
| `wordmark-white.svg`, `wordmark-white.png` | The logo with the name, on dark backgrounds |

The square icons keep the mark inside the circle that Google and Slack crop them to.

| Color | Hex |
|---|---|
| Ink | `#141413` |
| Paper | `#f7f8fa` |
| Blue | `#0066c5` |

Headlines are set in EB Garamond and text in Inter Tight.

`render.sh` renders every PNG again from the SVGs, with `rsvg-convert` (`brew install librsvg`).
