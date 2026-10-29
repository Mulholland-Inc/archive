#pragma frame {"size":[800,640],"center":[0.0,0.25,0.0],"zoom":235,"bounds":[[-1.2,-0.1,-1.2],[1.2,1.0,1.2]]}

vec2 base(vec3 p, float m) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.04, 0.0), 0.22, 0.04), WHITE + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.085, 0.0), 0.2, 0.012), m + ROUND));
  return r;
}

vec2 person(vec3 p) {
  vec2 r = base(p, LIME);
  r = U(r, M(sdCone(p - vec3(0.0, 0.2, 0.0), 0.1, 0.11, 0.06), LIME + ROUND));
  r = U(r, M(sdSphere(p - vec3(0.0, 0.38, 0.0), 0.075), LIME + SOLID));
  return r;
}

vec2 order(vec3 p) {
  vec2 r = base(p, LILAC);
  vec3 b = p - vec3(0.0, 0.22, 0.0);
  r = U(r, M(sdRoundBox(b, vec3(0.12, 0.12, 0.12), 0.015), WHITE));
  r = paint(r, abs(b.x) - 0.025, LILAC);
  return r;
}

vec2 product(vec3 p) {
  vec2 r = base(p, BLUE);
  r = U(r, M(sdHexPrism(p - vec3(0.0, 0.2, 0.0), 0.12, 0.1), BLUE));
  return r;
}

vec2 invoice(vec3 p) {
  vec2 r = base(p, STEEL);
  vec3 s = p - vec3(0.0, 0.25, 0.0);
  r = U(r, M(sdRoundBox(s, vec3(0.11, 0.15, 0.015), 0.006), WHITE));
  vec3 f = faceZ(s, 0.015);
  for (int i = 0; i < 4; i++) r = decal(r, f, sdRect(f.xz - vec2(-0.02 + float(i % 2) * 0.02, 0.08 - float(i) * 0.05), vec2(0.06 - float(i % 2) * 0.02, 0.011), 0.011), i == 0 ? LIME : STEEL);
  return r;
}

float link(vec3 p, vec3 a, vec3 b) { return sdCapsule(p, a, b, 0.022); }

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.0), vec3(1.0, 0.06, 1.0), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  vec3 g = faceY(p, 0.06);
  r = decal(r, g, abs(sdRect(g.xz, vec2(0.9, 0.9), 0.06)) - 0.006, STEEL);
  vec3 c = vec3(-0.5, 0.15, -0.5), o = vec3(0.5, 0.15, -0.5), pr = vec3(0.5, 0.15, 0.5), iv = vec3(-0.5, 0.15, 0.5);
  float l = min(min(link(p, c, o), link(p, o, pr)), min(link(p, o, iv), link(p, c, iv)));
  r = U(r, M(l, GRAPHITE + SOLID));
  r = U(r, person(p - vec3(-0.5, 0.06, -0.5)));
  r = U(r, order(p - vec3(0.5, 0.06, -0.5)));
  r = U(r, product(p - vec3(0.5, 0.06, 0.5)));
  r = U(r, invoice(p - vec3(-0.5, 0.06, 0.5)));
  return r;
}
