#pragma frame {"size":[800,640],"center":[0.0,0.32,0.0],"zoom":268,"bounds":[[-1.5,-0.1,-0.8],[1.5,1.2,0.8]]}

float sdBoxFrame(vec3 p, vec3 b, float e) {
  p = abs(p) - b;
  vec3 q = abs(p + e) - e;
  return min(min(
    length(max(vec3(p.x, q.y, q.z), 0.0)) + min(max(p.x, max(q.y, q.z)), 0.0),
    length(max(vec3(q.x, p.y, q.z), 0.0)) + min(max(q.x, max(p.y, q.z)), 0.0)),
    length(max(vec3(q.x, q.y, p.z), 0.0)) + min(max(q.x, max(q.y, p.z)), 0.0));
}

vec2 server(vec3 p) {
  vec3 b = p - vec3(0.0, 0.33, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.2, 0.33, 0.28), 0.025), WHITE);
  r = seam(r, b.y - 0.27);
  vec3 f = faceZ(p, 0.28);
  vec3 bay = vec3(f.x, f.y, repC(f.z - 0.36, 0.13, 3.0));
  r = inset(r, bay - vec3(-0.035, 0.0, 0.0), vec2(0.13, 0.045), 0.012, 0.015, STEEL);
  r = U(r, M(extrude(sdRect(bay.xz - vec2(-0.1, 0.0), vec2(0.012, 0.03), 0.012), bay.y + 0.004, 0.012), GRAPHITE));
  r = U(r, led(bay - vec3(0.145, 0.0, 0.0), 0.016, repI(f.z - 0.36, 0.13, 3.0) == 2.0 ? LIME : STEEL));
  r = vents(r, f - vec3(0.0, 0.0, 0.1), 6.0, 0.045, 0.1, 0.018, GRAPHITE);
  r = U(r, screw(vec3(abs(f.x) - 0.15, f.y, f.z - 0.62), 0.014));
  vec3 t = faceY(p, 0.66);
  r = vents(r, vec3(t.z, t.y, t.x) - vec3(0.0, 0.0, -0.02), 5.0, 0.07, 0.24, 0.02, GRAPHITE);
  vec3 fx = faceX(p, 0.2);
  r = U(r, M(extrude(sdRect(fx.xz - vec2(0.0, 0.33), vec2(0.2, 0.22), 0.02), fx.y - 0.006, 0.006), STEEL));
  r = U(r, screw(vec3(abs(fx.x) - 0.15, fx.y - 0.012, abs(fx.z - 0.33) - 0.17), 0.014));
  return r;
}

vec2 ghost(vec3 p) {
  vec3 b = p - vec3(0.0, 0.33, 0.0);
  vec2 r = M(sdBoxFrame(b, vec3(0.2, 0.33, 0.28), 0.012), WHITE + SOLID);
  return r;
}

vec2 map(vec3 p) {
  vec3 rb = p - vec3(0.0, 0.05, 0.0);
  vec2 r = M(sdRoundBox(rb, vec3(1.2, 0.05, 0.36), 0.02), WHITE);
  r = seam(r, rb.y - 0.01);
  vec3 top = faceY(p, 0.1);
  vec3 tq = vec3(top.x, top.y, abs(top.z) - 0.22);
  r = U(r, M(sdRoundBox(tq - vec3(0.0, 0.012, 0.0), vec3(1.16, 0.012, 0.025), 0.008), STEEL));
  vec3 fz = faceZ(p, 0.36);
  float i = repI(p.x, 0.54, 4.0);
  vec3 sq = vec3(repC(p.x, 0.54, 4.0), fz.y, fz.z - 0.05);
  r = U(r, led(sq - vec3(-0.12, 0.0, 0.0), 0.016, i == 0.0 ? LIME : STEEL));
  r = vents(r, sq - vec3(0.06, 0.0, 0.0), 4.0, 0.04, 0.02, 0.012, GRAPHITE);
  vec3 cs = mirX(fz) - vec3(1.13, 0.0, 0.05);
  r = U(r, screw(cs, 0.018));
  vec3 ec = vec3(abs(p.x) - 1.2, p.y - 0.12, p.z);
  r = U(r, M(sdRoundBox(ec, vec3(0.035, 0.12, 0.38), 0.02), STEEL));
  vec3 sp = vec3(repC(p.x, 0.54, 4.0), p.y - 0.1, p.z);
  r = U(r, server(vec3(p.x + 0.81, sp.yz)));
  r = U(r, ghost(vec3(repC(p.x - 0.27, 0.54, 3.0), sp.yz)));
  vec3 cl = vec3(sp.x, sp.y - 0.012, abs(sp.z) - 0.22);
  r = U(r, M(sdRoundBox(vec3(abs(cl.x) - 0.17, cl.y, cl.z), vec3(0.035, 0.02, 0.045), 0.008), i == 0.0 ? LIME : STEEL));
  return r;
}
