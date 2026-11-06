#pragma frame {"size":[1920,1080],"center":[0.0,0.9,0.0],"zoom":190,"bounds":[[-4,-0.6,-4],[4,6,4]],"t":3}

vec2 fence(vec3 p, float t0) {
  vec3 q = vec3(abs(p.x), p.y, abs(p.z));
  vec3 a = vec3(repC(p.x, 0.42, 13.0), p.y, q.z - 2.55);
  vec3 b = vec3(q.x - 2.55, p.y, repC(p.z, 0.42, 13.0));
  float ia = repI(p.x, 0.42, 13.0), ib = repI(p.z, 0.42, 13.0);
  float ha = 0.3 * pop(t0 + ia * 0.022 + (p.z > 0.0 ? 0.0 : 0.15), 0.25);
  float hb = 0.3 * pop(t0 + ib * 0.022 + (p.x > 0.0 ? 0.08 : 0.2), 0.25);
  float d = 1e9;
  if (ha > 0.01) d = min(d, sdBox(a - vec3(0.0, ha, 0.0), vec3(0.06, ha, 0.06)));
  if (hb > 0.01) d = min(d, sdBox(b - vec3(0.0, hb, 0.0), vec3(0.06, hb, 0.06)));
  vec2 r = M(d, WHITE);
  float l = glide(t0 + 0.4, 0.3) * 2.55;
  if (l > 0.01) r = U(r, M(min(sdBox(vec3(p.x, p.y - 0.4, q.z - 2.55), vec3(l, 0.04, 0.03)), sdBox(vec3(q.x - 2.55, p.y - 0.4, p.z), vec3(0.03, 0.04, l))), STEEL));
  return r;
}

vec2 map(vec3 p) {
  float rise = pop(0.0, 0.3);
  vec3 g = p - vec3(0.0, -0.14, 0.0);
  float s = max(rise, 0.01);
  vec2 r = M(sdRoundBox(g / vec3(s, 1.0, s), vec3(2.8, 0.14, 2.8), 0.02) * s, WHITE);
  r = seam(r, g.y + 0.07);
  r = paint(r, g.y < 0.13 ? 1.0 : sdRect(p.xz, vec2(2.35), 0.1), MINT);
  r = U(r, fence(p, 0.2));
  for (int i = 0; i < 3; i++) {
    float fi = float(i), t0 = 0.45 + fi * 0.16;
    float x = ramp(t0, 0.3);
    if (x <= 0.0) continue;
    float y = 0.42 + fi * 0.8 + 5.0 * (1.0 - x) * (1.0 - x);
    r = U(r, fSlice(p - vec3(0.0, y, 0.0)));
  }
  if (T > 0.3) r = U(r, M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.95 * pop(0.3, 0.2), 0.03), GRAPHITE + ROUND));
  if (T > 1.1) {
    float x = clamp((T - 1.1) / 0.7, 0.0, 1.0), e = (1.0 - x) * (1.0 - x);
    float y = 0.14 + 3.4 * abs(cos(x * 7.85)) * e;
    vec3 q = rotY(p - vec3(1.45, y, 1.45), 0.6 + 6.0 * e + 0.25 * T);
    r = U(r, fKey(q, BLUE));
  }
  return r;
}
