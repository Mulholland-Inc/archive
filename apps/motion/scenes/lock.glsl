#pragma frame {"size":[1920,1080],"center":[0.0,1.7,0.0],"zoom":260,"yaw":32,"pitch":14,"bounds":[[-9,-0.5,-5],[9,9,8]],"t":2}

vec2 leaf(vec3 p) {
  vec3 c = p - vec3(1.1, 1.8, 0.0);
  vec2 r = M(sdBox(c, vec3(1.08, 1.78, 0.08)), LIME);
  vec2 q = vec2(c.x, abs(c.y) - 0.9);
  if (c.z > 0.07) r = seam(r, sdRect(q, vec2(0.8, 0.68), 0.0));
  vec3 l = p - vec3(1.72, 1.8, 0.1);
  r = U(r, M(sdRoundBox(l, vec3(0.2, 0.34, 0.04), 0.02), WHITE));
  r = paint(r, max(l.z - 0.02 < 0.0 ? 1.0 : -1.0, min(sdCircle(l.xy - vec2(0.0, 0.05), 0.07), sdRect(l.xy + vec2(0.0, 0.08), vec2(0.03, 0.1), 0.0))), GRAPHITE);
  float enter = 1.0 - glide(0.05, 0.4);
  float turn = glide(0.55, 0.28) * 1.5708;
  vec3 k = l - vec3(0.0, 0.05, 0.75 + 2.6 * enter);
  k.xy = rot(turn) * k.xy;
  vec3 kq = vec3(-k.z, k.x, k.y);
  r = U(r, fKey(kq / 0.9, WHITE) * vec2(0.9, 1.0));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 4.0, -0.3), vec3(9.0, 4.0, 0.3)), WHITE);
  r = stripes(r, p.y, 0.45);
  r.y = mod(r.y, 16.0);
  r.x = max(r.x, -sdBox(p - vec3(0.0, 1.8, -0.3), vec3(1.1, 1.8, 0.5)));
  r = U(r, M(max(sdBox(p - vec3(0.0, 1.9, -0.3 + 0.05), vec3(1.26, 1.92, 0.35)), -sdBox(p - vec3(0.0, 1.8, -0.3), vec3(1.1, 1.8, 0.6))), GRAPHITE));
  float swing = glide(1.0, 0.5) * 1.75;
  vec3 h = p - vec3(-1.1, 0.0, 0.0);
  r = U(r, leaf(rotY(h, swing)));
  return r;
}
