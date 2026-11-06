#pragma frame {"size":[1920,1080],"center":[0.0,0.75,0.0],"zoom":120,"yaw":28,"pitch":22,"bounds":[[-30,-0.5,-3],[30,14,4]],"t":2}

float hash(vec2 n) { return fract(sin(dot(n, vec2(12.9898, 78.233))) * 43758.5453); }

vec2 map(vec3 p) {
  float cx = round(p.x / 2.0), cy = round((p.y - 0.8) / 1.7);
  cy = max(cy, 0.0);
  vec3 q = vec3(p.x - cx * 2.0, p.y - 0.8 - cy * 1.7, p.z);
  bool mine = cx == 0.0 && cy == 0.0;
  float shut = mine ? glide(0.55, 0.45) : 0.75 + 0.25 * sin(T * 3.0 + hash(vec2(cx, cy)) * 40.0);
  float wall = sdBox(p - vec3(0.0, 6.0, -1.0), vec3(40.0, 6.0, 1.0));
  vec2 r = M(wall, WHITE);
  if (mine) r.x = max(r.x, -sdBox(q - vec3(0.0, 0.0, -0.5), vec3(0.86, 0.72, 0.6)));
  r = paint(r, sdRect(q.xy, vec2(0.86, 0.72), 0.0), GRAPHITE);
  if (abs(p.z) < 0.03 && abs(q.x) < 0.86 && abs(q.y) < 0.72) r = seam(r, 0.0 * q.x + (abs(q.x) > 0.84 ? 1.0 : -1.0));
  float top = 0.72 - 1.44 * shut;
  vec2 door = M(sdBox(q - vec3(0.0, 0.0, 0.03), vec3(0.84, 0.7, 0.02)), mine ? LIME : STEEL);
  door.x = max(door.x, top - q.y);
  door = stripes(door, q.y, 0.09);
  r = U(r, door);
  if (mine) {
    vec3 d = p - vec3(0.0, 0.1, -0.5);
    r = U(r, fDrum(d / 0.48) * vec2(0.48, 1.0));
    float l = pop(0.95, 0.3);
    if (l > 0.01) {
      vec3 c = q - vec3(0.0, -0.62, 0.1);
      vec2 lock = M(sdRoundBox(c / l, vec3(0.16, 0.14, 0.06), 0.02) * l, GRAPHITE);
      lock = U(lock, M(sdTorus((c - vec3(0.0, 0.14 * l, 0.0)).xzy, 0.1 * l, 0.028 * l), STEEL + ROUND));
      r = U(r, lock);
    }
  }
  return r;
}
