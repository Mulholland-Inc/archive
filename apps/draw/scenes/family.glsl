#pragma frame {"size":[800,640],"center":[0.0,0.6,0.1],"zoom":215,"bounds":[[-1.15,-0.05,-0.8],[1.15,1.6,1.1]]}

float gable(vec3 q, float h, float w) {
  return (h * abs(q.z) + w * q.y - h * w) / length(vec2(h, w));
}

vec2 house(vec3 p) {
  vec3 b = p - vec3(0.0, 0.5, 0.0);
  vec2 r = M(sdBox(b, vec3(0.55, 0.42, 0.32)), WHITE);
  vec3 g = p - vec3(0.0, 0.92, 0.0);
  r = U(r, M(max(max(gable(g, 0.38, 0.32), -g.y), abs(g.x) - 0.55), WHITE));
  float d = gable(g - vec3(0.0, 0.03, 0.0), 0.38, 0.32);
  r = U(r, M(max(max(d, -(d + 0.05)), max(abs(g.x) - 0.62, max(abs(g.z) - 0.42, -g.y - 0.08))), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.28, 1.12, -0.14), vec3(0.07, 0.17, 0.07)), STEEL));
  r = U(r, M(sdBox(p - vec3(0.28, 1.3, -0.14), vec3(0.09, 0.02, 0.09)), STEEL));

  vec3 f = faceZ(p, 0.32);
  r = inset(r, f - vec3(0.0, 0.0, 0.3), vec2(0.1, 0.21), 0.02, 0.012, LIME);
  r = U(r, M(sdSphere(p - vec3(0.06, 0.3, 0.335), 0.018), STEEL));
  r = inset(r, f - vec3(-0.34, 0.0, 0.56), vec2(0.12, 0.12), 0.02, 0.012, SKY);
  r = inset(r, f - vec3(0.34, 0.0, 0.56), vec2(0.12, 0.12), 0.02, 0.012, SKY);

  vec3 x = faceX(p, 0.55);
  r = inset(r, x - vec3(0.0, 0.0, 0.58), vec2(0.14, 0.13), 0.02, 0.012, SKY);
  r = inset(r, x - vec3(0.0, 0.0, 1.08), vec2(0.07, 0.07), 0.07, 0.012, SKY);
  r = U(r, M(sdBox(p - vec3(0.0, 0.1, 0.0), vec3(0.57, 0.02, 0.34)), GRAPHITE));
  return r;
}

vec2 mailbox(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.2, 0.0), vec3(0.025, 0.2, 0.025)), GRAPHITE);
  vec3 b = p - vec3(0.0, 0.46, 0.0);
  vec2 box = M(max(sdCylinder(rotX(b, PI * 0.5), 0.08, 0.14), -b.y - 0.06), LIME + ROUND);
  box = U(box, M(sdBox(b - vec3(0.0, -0.03, 0.0), vec3(0.08, 0.03, 0.14)), LIME));
  r = U(r, box);
  r = U(r, M(sdBox(b - vec3(0.09, 0.06, -0.05), vec3(0.008, 0.07, 0.016)), STEEL));
  r = U(r, M(sdBox(rotX(b - vec3(0.0, 0.07, 0.15), -0.3), vec3(0.06, 0.04, 0.004)), WHITE));
  return r;
}

vec2 tree(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.16, 0.0), 0.025, 0.16), GRAPHITE + ROUND);
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.5, 0.0), vec3(0.19, 0.24, 0.19)), MINT + SOLID));
  return r;
}

vec2 album(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.17, 0.03, 0.13), 0.008), BLUE);
  r = U(r, M(sdBox(p - vec3(0.01, 0.03, 0.0), vec3(0.155, 0.022, 0.13)), WHITE));
  vec3 t = rotY(p - vec3(0.02, 0.085, 0.0), 0.25);
  r = U(r, M(sdRoundBox(t, vec3(0.15, 0.025, 0.11), 0.008), LILAC));
  vec3 ph = rotY(p - vec3(0.12, 0.118, 0.1), -0.3);
  r = U(r, M(sdBox(ph, vec3(0.08, 0.006, 0.065)), WHITE));
  r = decal(r, faceY(ph, 0.006), sdRect(ph.xz, vec2(0.06, 0.045), 0.004), SKY);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.15), vec3(1.05, 0.06, 0.85), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  r = U(r, house(p - vec3(0.0, 0.0, -0.1)));
  r = U(r, mailbox(p - vec3(0.82, 0.06, 0.72)));
  r = U(r, tree(p - vec3(-0.8, 0.06, 0.5)));
  r = U(r, album(p - vec3(0.05, 0.06, 0.74)));
  return r;
}
