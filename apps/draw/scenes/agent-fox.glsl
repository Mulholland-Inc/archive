#pragma frame {"label":"Fox"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  float f = smin(sdEllipsoid(p - V(0., 1., 0.), V(.66, .56, .5)), sdEllipsoid(mirX(p) - V(.5, .82, 0.), V(.34, .22, .24)), .2);
  vec2 h = M(f, MID);
  h = dab(h, m, vec2(.34, .76), .3, PALE);
  h = oblong(h, p, vec2(0., .72), vec2(.2, .18), .17, PALE);
  h = eyes(h, p, .28, 1.04, .15);
  h = dab(h, p, vec2(0., .78), .055, DARK);
  vec2 r = h;
  r = U(r, M(sdCone(rotZ(m - V(.44, 1.62, 0.), -.28), .3, .24, .015), MID));
  r = U(r, oval(p, V(0., .28, -.02), V(.34, .3, .28), MID));
  vec3 q = rotZ(p - V(.62, .42, -.2), -.7);
  r = U(r, q.y > .12 ? oval(q, V(0.), V(.2, .44, .2), PALE) : oval(q, V(0.), V(.2, .44, .2), TONE));
  r = U(r, oval(m, V(.17, .06, .1), V(.12, .06, .15), TONE));
  return r;
}
