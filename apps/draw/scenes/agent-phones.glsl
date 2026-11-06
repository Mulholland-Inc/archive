#pragma frame {"label":"Headphone robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .96, 0.), .64, PALE);
  h = oblong(h, p, vec2(0., .98), vec2(.42, .22), .15, DARK);
  h = dab(h, m, vec2(.2, .98), .125, EYE);
  vec2 r = h;
  r = U(r, canX(m, V(.68, .96, 0.), .26, .1, TONE));
  vec3 q = p - V(0., .96, 0.);
  r = U(r, M(max(sdTorus(q.xzy, .72, .05), -q.y), MID));
  r = U(r, stick(p, V(-.68, .78, .14), V(-.3, .46, .5), .022, MID));
  r = U(r, box(p, V(0., .18, 0.), V(.26, .16, .2), .06, SHELL));
  return r;
}
