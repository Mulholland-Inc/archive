#pragma frame {"size":[640,840],"center":[0.0,0.9,0.0],"zoom":240,"bounds":[[-24,-0.4,-3],[24,4,3]],"t":2}

vec2 peg(vec3 p) {
  vec3 b = p - vec3(0.0, 0.25, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.9, 0.25, 0.9), 0.02), WHITE);
  r = Sc(r, M(sdCylinder(b - vec3(0.0, 0.2, 0.0), 0.34, 0.3), GRAPHITE + ROUND));
  float y = 1.42 + 0.75 * abs(cos(T * 3.4)) - 0.08 * (1.0 - abs(cos(T * 3.4)));
  vec3 q = rotY(p - vec3(0.0, y, 0.0), 0.12 * sin(T * 11.0));
  return U(r, M(sdRoundBox(q, vec3(0.42, 0.84, 0.42), 0.02), LIME));
}

vec2 lid(vec3 p) {
  vec3 pl = p - vec3(0.0, 0.1, 0.0);
  vec2 r = M(sdRoundBox(pl, vec3(0.95, 0.1, 0.95), 0.02), WHITE);
  float over = max(T * 1.3 - 0.95, 0.0);
  float h = min(T * 1.3, 0.95);
  float w = 0.42 + 0.2 * min(over, 1.0);
  float shake = over > 0.0 ? 0.02 * sin(T * 50.0) * exp(-over * 1.5) : 0.0;
  vec3 tw = p - vec3(0.0, 0.2 + h, 0.0);
  vec2 s = M(sdRoundBox(tw, vec3(w, h, w), 0.02), LIME);
  s = stripes(s, p.y - 0.2, 0.3);
  r = U(r, s);
  vec3 pc = mirXZ(p - vec3(0.0, 0.0, 0.0));
  r = U(r, M(sdBox(pc - vec3(0.86, 1.1, 0.86), vec3(0.05, 0.95, 0.05)), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 2.2 + shake, 0.0), vec3(1.0, 0.12, 1.0), 0.02), GRAPHITE));
  return r;
}

vec2 gate(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.08, 0.0), vec3(2.1, 0.08, 0.5), 0.02), STEEL);
  r = stripes(r, p.x + T * 2.0, 0.2);
  float drop = glide(0.25, 0.3);
  for (int i = 0; i < 4; i++) {
    float fi = float(i);
    float x = min(-2.6 + mod(T * 2.6, 0.01) + T * 2.6 - fi * 0.72, 0.35 - fi * 0.66);
    vec3 q = p - vec3(x, 0.44, 0.0);
    r = U(r, M(sdRoundBox(q, vec3(0.28, 0.28, 0.28), 0.02), i == 0 ? LIME : i == 1 ? LILAC : WHITE));
  }
  vec3 a = p - vec3(0.86, 0.8, -0.72);
  r = U(r, M(sdBox(p - vec3(0.86, 0.45, -0.72), vec3(0.07, 0.45, 0.07)), GRAPHITE));
  vec3 arm = rotX(a, -(1.0 - drop) * 1.4);
  vec2 b = M(sdBox(arm - vec3(0.0, 0.0, 0.8), vec3(0.04, 0.04, 0.8)), WHITE);
  b = stripes(b, arm.z, 0.16);
  b.y = mod(arm.z, 0.32) < 0.16 ? BLUE : WHITE;
  return U(r, b);
}

vec2 map(vec3 p) {
  float k = clamp(round(p.x / 16.0), -1.0, 1.0);
  vec3 q = p - vec3(k * 16.0, 0.0, 0.0);
  vec2 r = k < -0.5 ? peg(q) : k < 0.5 ? lid(q) : gate(q);
  r.x = min(r.x, max(8.0 - abs(q.x), 0.0) + 0.5);
  return r;
}
