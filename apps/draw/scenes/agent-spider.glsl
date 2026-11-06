#pragma frame {"label":"Spider robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .86, 0.), V(.72, .54, .6), PALE);
  h = oblong(h, p, vec2(0., .9), vec2(.5, .27), .22, DARK);
  h = dab(h, m, vec2(.24, .96), .12, EYE);
  h = dab(h, m, vec2(.09, .76), .05, EYE);
  vec2 r = h;
  vec3 q = vec3(abs(p.x), p.y, abs(p.z));
  r = U(r, stick(q, V(.5, .62, .36), V(.82, .72, .54), .05, MID));
  r = U(r, stick(q, V(.82, .72, .54), V(.92, .06, .62), .045, TONE));
  r = U(r, ball(q, V(.82, .72, .54), .08, TONE));
  r = U(r, stick(p, V(0., 1.38, 0.), V(0., 1.5, 0.), .03, TONE));
  r = U(r, ball(p, V(0., 1.54, 0.), .06, PALE));
  return r;
}
