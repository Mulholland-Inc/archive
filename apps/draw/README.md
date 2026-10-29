# draw

Isometric illustrations for the marketing site: signed distance field scenes raymarched by a WebGL2 shader into flat fills with uniform ink outlines.

- Preview: `cd apps/draw && python3 -m http.server`, open http://localhost:8000. Pick a scene; it re-renders when a scene or `sdf.glsl` changes on disk.
- Author: add `scenes/<name>.glsl` with a first line `#pragma frame {"size":[w,h],"center":[x,y,z],"zoom":px_per_unit}` (optional `yaw`, `pitch`, `bounds`, `shade`, `trim`) and a `vec2 map(vec3 p)` that returns `vec2(distance, material)`. Compose the helpers in `sdf.glsl`; materials are the palette names in capitals (`WHITE`, `LIME`, ...) plus `ROUND` for cylinders and `SOLID` for one fill with no internal face edges.
- Bake: `node apps/draw/bake.mjs <scene|a,b|all> [--scale 2] [--ss 3] [--line 1] [--bg #hex] [--shade 0..1] [--out dir]` writes transparent `apps/site/static/art/<name>.webp`, trimmed to the drawing with a small margin unless the frame sets `"trim":false`, using headless Chrome (pass `--png` to keep the PNG too; without `cwebp` only the PNG is written).
- Palette: `palette.json`, the same tokens as the site.
