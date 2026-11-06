# motion

A live-editing motion graphics engine. Source is edited on the Mac; a RunPod GPU pod renders it; only shots whose inputs changed are rendered again.

Needs on the Mac: node, rsync, ssh, gcloud access to `RUNPOD_API_KEY` in `marzy-agent`, and `~/.ssh/id_ed25519.pub`. Run commands as `node apps/motion/motion.mjs <command>`.

## A session

- `up`: rents one pod, installs the toolchain and warms Chrome workers. Refuses if any pod exists. Moves on from a host that refuses NVENC; if none has it, it stops and says so. `MOTION_CPU=1` accepts libx264 for that session.
- `render [shot...]`: syncs, renders changed shots, joins, downloads to `out/film.mp4` (or `out/<shots>.mp4`). Prints time per stage.
- `frame <seconds>`: one frame to `out/frame.png`.
- `sheet <from> <to> [count]`: a labelled contact sheet to `out/sheet.png`. Review with these before rendering.
- `status`: pods, uptime, cost so far, idle time.
- `logs`: the pod server's log.
- `down`: deletes the pod and confirms none remain. Always finish with this.

A watchdog on the pod deletes it after 30 minutes without a render, frame or sheet (`MOTION_IDLE` seconds at `up`).

## Directing a film

`film.js` is the film: everything before the first `shot(` is shared, then one block per shot, each starting at column 0. The film may run at most 60.0 s.

```js
shot('reach', 2.5, 'left', s => {
  const cam = s.cam({ center: [-3.3, 1.1, 0], zoom: 560 }, { center: [1.2, 2.6, 0], zoom: 142 }, 0.2, 1.5);
  const head = s.type(['Your AI can’t reach', 'your data directly.'], { x: 96, y: 125, size: 120, at: 1.1 });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'straw', t, cam());
    head(g);
  };
});
```

The builder runs once and declares motion as GSAP tweens on plain values; the returned function draws a frame from `t` and those values, so it must keep no state between frames. A shot with no motion is refused. The optional third argument is how the shot enters: `left`, `right`, `up`, `down`, `iris`.

Vocabulary, in `kit.js`:

- `s.v(at, d, ease, from, to)`: a value, read as `.v`. `s.keys(from, [[to, d, ease, delay], ...])`: several in turn.
- `s.cam(a, b, at, d, ease)`: a camera move; call it for the camera now.
- `s.type(lines, opts)`: masked type, word by word. `opts.hot` colours words; `opts.leave` sends them out.
- `s.swap([[word, at], ...], opts)`, `s.strike(rows, opts)`, `s.bar(...)`: word swaps, strikes, wipes.
- `curve(g, pts, p)`: a line or chart drawn on to progress `p`. `tile`, `logo`, `wordmark`, `shifted`, `project`, `zoomed`.
- Eases: `snap`, `quart`, `soft`, `glide`, `settle`, `pop`, `in`, `linear`, `spring`, `bounce`.

Scenes are one file each in `scenes/`: a `#pragma frame {...}` line and `vec2 map(vec3 p)`, built on `apps/draw/sdf.glsl` and `film.glsl`, with `T` as time. `art(g, 'name', t, cam)` draws one; write its name as a quoted literal in the shot, because that is how the cache sees the dependency. Anything else is refused.

Type is designed into the shot, not subtitled. Every shot has a full sentence, words are literal, claims stay within what the site says, and charts carry no invented numbers.

## The cache

A shot's key hashes its block, the scenes it names, the shared preamble of `film.js`, `engine.js`, `kit.js`, the GLSL libraries and palette if it uses a scene, and the encoder settings. A shot that enters over the previous one also depends on that shot. So editing a sentence re-renders one shot, editing `sdf.glsl` re-renders every shot with a scene, and editing `kit.js` re-renders everything.

## Errors

GLSL errors report file, line and column. Timeline errors report the shot and its time. A syntax error in `film.js` reports its line and column.
