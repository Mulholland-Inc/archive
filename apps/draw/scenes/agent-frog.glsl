#pragma frame {"label":"Frog"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .68, 0.), V(.86, .56, .6), PALE);
  h = dab(h, p, vec2(0., .24), .4, MID);
  h = smile(h, p, vec2(0., 1.02), .5, .018, DARK);
  vec2 e = ball(m, V(.42, 1.14, .1), .3, PALE);
  e = eyes(e, p, .42, 1.18, .2);
  vec2 r = U(h, e);
  r = U(r, oval(m, V(.46, .08, .3), V(.26, .08, .2), TONE));
  r = U(r, oval(m, V(.72, .34, .14), V(.16, .26, .22), TONE));
  return r;
}
