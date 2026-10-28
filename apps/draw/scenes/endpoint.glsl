#pragma frame {"size":[800,640],"center":[-0.1,0.42,0.42],"zoom":245,"bounds":[[-0.8,-0.1,-0.8],[1.7,1.4,1.7]]}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float k = 1e9;
  vec3 prev = a;
  for (int i = 1; i <= 24; i++) {
    float t = float(i) / 24.0;
    vec3 ab = mix(a, b, t), bc = mix(b, c, t), cd = mix(c, d, t);
    vec3 q = mix(mix(ab, bc, t), mix(bc, cd, t), t);
    k = min(k, sdCapsule(p, prev, q, r));
    prev = q;
  }
  return k;
}

vec2 dial(vec3 q) {
  vec2 r = M(sdCylinder(q - vec3(0.0, 0.025, 0.0), 0.2, 0.025), STEEL + ROUND);
  vec3 f = q - vec3(0.0, 0.05, 0.0);
  r = Sc(r, M(max(length(f.xz) - 0.165, -f.y - 0.015), WHITE));
  float a = atan(f.z, f.x);
  float rr = length(f.xz);
  float arc = max(abs(rr - 0.135) - 0.016, f.y + 0.014);
  float up = -f.z - 0.02;
  if (max(arc, up) < 0.0 && f.y > -0.016) {
    float m = a > 2.1 ? LIME : a > 1.05 ? SUN : CORAL;
    r = M(r.x, m);
  }
  float st = PI / 8.0;
  float an = mod(a + st * 0.5, st) - st * 0.5;
  vec2 tp = vec2(rr * cos(an), rr * sin(an));
  float tick = max(sdRect(tp - vec2(0.1, 0.0), vec2(0.014, 0.004), 0.002), -f.z + 0.005);
  r = U(r, M(extrude(tick, f.y + 0.012, 0.004), GRAPHITE + SOLID));
  vec2 nd = vec2(cos(0.75), sin(0.75));
  float needle = sdSeg(f.xz, vec2(0.0), nd * 0.13) - 0.009;
  r = U(r, M(extrude(needle, f.y + 0.006, 0.006), CORAL + SOLID));
  r = U(r, M(sdCylinder(f - vec3(0.0, 0.0, 0.0), 0.025, 0.012), GRAPHITE + ROUND));
  return r;
}

vec2 server(vec3 p) {
  vec3 fe = mirXZ(p) - vec3(0.4, 0.03, 0.32);
  vec2 r = M(sdRoundCylinder(fe, 0.05, 0.03, 0.01), GRAPHITE + ROUND);
  vec3 b = p - vec3(0.0, 0.4, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.5, 0.34, 0.42), 0.04), WHITE);
  body = seam(body, b.y - 0.24);
  body = seam(body, b.y + 0.26);
  vec3 fz = faceZ(p, 0.42);
  body = vents(body, fz - vec3(0.28, 0.0, 0.34), 5.0, 0.05, 0.16, 0.022, GRAPHITE);
  vec3 fx = faceX(p, 0.5);
  body = grille(body, fx - vec3(-0.12, 0.0, 0.4), vec2(5.0, 4.0), 0.06, 0.016, GRAPHITE);
  r = U(r, body);
  r = U(r, dial(fz - vec3(-0.15, 0.0, 0.38)));
  r = U(r, led(fz - vec3(0.28, 0.0, 0.53), 0.032, LIME));
  r = U(r, led(fz - vec3(0.38, 0.0, 0.53), 0.032, STEEL));
  r = U(r, button(fz - vec3(0.18, 0.012, 0.53), 0.03, 0.012, GRAPHITE));
  r = U(r, screw(vec3(fz.x - 0.43, fz.y, fz.z - 0.2), 0.018));
  r = U(r, screw(vec3(fz.x + 0.43, fz.y, fz.z - 0.2), 0.018));
  r = U(r, screw(vec3(fz.x - 0.43, fz.y, fz.z - 0.58), 0.018));
  r = U(r, screw(vec3(fz.x + 0.43, fz.y, fz.z - 0.58), 0.018));
  r = U(r, M(sdCylinder(fx - vec3(0.3, 0.03, 0.2), 0.07, 0.03), STEEL + ROUND));
  r = U(r, screw(fx - vec3(0.3, 0.0, 0.58), 0.018));
  r = U(r, screw(fx - vec3(-0.3, 0.0, 0.2), 0.018));
  vec3 tp = faceY(p, 0.74);
  r = U(r, M(sdRoundBox(tp - vec3(0.0, 0.025, 0.06), vec3(0.22, 0.025, 0.035), 0.012), STEEL));
  r = U(r, M(sdBox(vec3(abs(tp.x) - 0.2, tp.y - 0.02, tp.z - 0.06), vec3(0.02, 0.02, 0.06)), STEEL));
  vec3 an = tp - vec3(-0.34, 0.0, -0.26);
  r = U(r, M(sdRoundCylinder(an - vec3(0.0, 0.03, 0.0), 0.05, 0.03, 0.01), STEEL + ROUND));
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.14, 0.0), 0.011, 0.11), GRAPHITE + ROUND));
  r = U(r, M(sdSphere(an - vec3(0.0, 0.27, 0.0), 0.032), CORAL + SOLID));
  return r;
}

vec2 socket(vec3 p) {
  vec3 q = p - vec3(0.0, 0.12, 0.0);
  vec2 r = M(sdRoundBox(q, vec3(0.15, 0.12, 0.1), 0.02), WHITE);
  r = seam(r, q.y + 0.07);
  vec3 f = faceZ(q, 0.1);
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.0, 0.02), vec2(0.1, 0.075), 0.03), f.y - 0.008, 0.008), LIME));
  vec3 s = vec3(abs(f.x) - 0.04, f.y - 0.016, f.z - 0.02);
  r = Sc(r, M(max(length(s.xz) - 0.02, -(s.y + 0.04)), GRAPHITE));
  r = U(r, screw(faceY(q, 0.12) - vec3(0.0, 0.0, 0.0), 0.018));
  return r;
}

vec2 plug(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.08, 0.065, 0.1), 0.02), STEEL);
  r = seam(r, p.z + 0.05);
  vec3 pr = vec3(abs(p.x) - 0.04, p.y - 0.02, p.z + 0.14);
  r = U(r, M(sdCylinder(rotX(pr, PI * 0.5), 0.011, 0.045), STEEL + ROUND));
  r = U(r, M(sdCylinder(rotX(p - vec3(0.0, 0.0, 0.13), PI * 0.5), 0.05, 0.035), GRAPHITE + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.6, 0.0), vec3(0.6, 0.6, 0.5), r.x)) r = U(r, server(p));
  vec3 sp = rotY(p - vec3(-0.5, 0.0, 1.2), PI * 0.5);
  if (near(sp - vec3(0.0, 0.13, 0.0), vec3(0.2, 0.15, 0.2), r.x)) r = U(r, socket(sp));
  vec3 pp = rotY(p - vec3(-0.12, 0.12, 1.2), PI * 0.5);
  if (near(pp, vec3(0.12, 0.1, 0.25), r.x)) r = U(r, plug(pp));
  if (near(p - vec3(0.55, 0.15, 0.8), vec3(0.6, 0.2, 0.65), r.x)) {
    r = U(r, M(cable(p, vec3(0.56, 0.2, 0.3), vec3(1.15, 0.2, 0.3), vec3(0.85, 0.06, 1.25), vec3(0.03, 0.12, 1.2), 0.045), GRAPHITE + SOLID));
  }
  return r;
}
