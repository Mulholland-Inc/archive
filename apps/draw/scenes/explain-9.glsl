#pragma frame {"size":[800,640],"center":[0.0,0.25,0.0],"zoom":235,"bounds":[[-1.2,-0.1,-1.2],[1.2,1.2,1.2]]}

vec2 tile(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.02, 0.0), vec3(0.17, 0.02, 0.15), 0.015), STEEL);
  float row = repI(p.z, 0.09, 3.0);
  vec3 c = vec3(repC(p.x, 0.11, 3.0), p.y, repC(p.z, 0.09, 3.0));
  r = U(r, M(sdRoundBox(c - vec3(0.0, 0.06, 0.0), vec3(0.045, 0.02, 0.035), 0.008), row == 0.0 ? LILAC : WHITE));
  return r;
}

vec2 drum(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 3; i++) {
    float y = 0.07 + float(i) * 0.12;
    r = U(r, M(sdCylinder(p - vec3(0.0, y, 0.0), 0.13, 0.05), WHITE + ROUND));
    r = U(r, M(sdCylinder(p - vec3(0.0, y + 0.055, 0.0), 0.12, 0.008), LIME + ROUND));
  }
  return r;
}

vec2 code(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.12, 0.03, 0.06), 0.01), GRAPHITE);
  vec3 s = p - vec3(0.0, 0.2, 0.0);
  r = U(r, M(sdRoundBox(s, vec3(0.17, 0.13, 0.02), 0.01), GRAPHITE));
  vec3 f = faceZ(s, 0.02);
  for (int i = 0; i < 4; i++) {
    float l = 0.06 + 0.03 * mod(float(i) * 2.0, 3.0);
    float x0 = -0.12 + (i == 1 || i == 2 ? 0.04 : 0.0);
    r = decal(r, f, sdRect(f.xz - vec2(x0 + l, 0.075 - float(i) * 0.05), vec2(l, 0.011), 0.011), i == 0 ? LIME : i == 2 ? LILAC : WHITE);
  }
  return r;
}

vec2 key(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.08, 0.0), vec3(0.11, 0.08, 0.11), 0.015), BLUE);
  vec3 k = p - vec3(0.0, 0.16, 0.0);
  r = U(r, M(sdBox(k - vec3(0.0, 0.08, 0.0), vec3(0.015, 0.08, 0.015)), STEEL));
  r = U(r, M(sdTorus(rotX(k - vec3(0.0, 0.22, 0.0), PI * 0.5).xzy, 0.06, 0.02), STEEL));
  return r;
}

vec2 bot(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.07, 0.0), vec3(0.08, 0.07, 0.08), 0.025), STEEL);
  vec3 h = p - vec3(0.0, 0.24, 0.0);
  r = U(r, M(sdRoundBox(h, vec3(0.12, 0.1, 0.12), 0.035), WHITE));
  vec3 f = faceX(h, 0.12);
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.0, 0.02), vec2(0.085, 0.045), 0.035), f.y, 0.008), GRAPHITE));
  r = U(r, led(vec3(abs(f.x) - 0.04, f.y - 0.008, f.z - 0.02), 0.018, LILAC));
  r = U(r, M(sdSphere(h - vec3(0.0, 0.15, 0.0), 0.03), LIME + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.06, 0.0), 1.0, 0.06), WHITE + ROUND);
  r = seam(r, -(p.y + 0.02));
  vec3 g = faceY(p, 0.12);
  float a = atan(g.z, g.x) + PI * 0.1;
  float sec = mod(a + TAU, TAU / 5.0) - TAU / 10.0;
  vec2 rd = length(g.xz) * vec2(cos(sec), sin(sec));
  r = decal(r, g, max(abs(rd.y) - 0.008, -(rd.x - 0.28)), STEEL);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.16, 0.0), 0.2, 0.04), LIME + ROUND));
  float R = 0.62;
  vec3 c0 = vec3(R * cos(0.0), 0.12, R * sin(0.0));
  vec3 c1 = vec3(R * cos(TAU / 5.0), 0.12, R * sin(TAU / 5.0));
  vec3 c2 = vec3(R * cos(2.0 * TAU / 5.0), 0.12, R * sin(2.0 * TAU / 5.0));
  vec3 c3 = vec3(R * cos(3.0 * TAU / 5.0), 0.12, R * sin(3.0 * TAU / 5.0));
  vec3 c4 = vec3(R * cos(4.0 * TAU / 5.0), 0.12, R * sin(4.0 * TAU / 5.0));
  r = U(r, tile(p - c0));
  r = U(r, drum(p - c1));
  r = U(r, code(p - c2));
  r = U(r, key(p - c3));
  r = U(r, bot(p - c4));
  return r;
}
