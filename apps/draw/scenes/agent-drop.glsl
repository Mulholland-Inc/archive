#pragma frame {"label":"Water drop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec3 q = p - V(0., .62, 0.);
  float tip = sdCapsule(p, V(0., 1.6, 0.), V(0., .7, 0.), .02);
  vec2 b = M(smin(sdSphere(q, .62), tip, .72), PALE);
  b = eyes(b, p, .24, .7, .17);
  b = line(b, m, vec2(.0, .42), vec2(.1, .46), .012, TONE);
  vec2 r = b;
  r = U(r, oval(m, V(.42, .06, .12), V(.16, .07, .14), MID));
  return r;
}
