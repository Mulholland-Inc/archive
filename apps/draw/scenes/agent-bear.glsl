#pragma frame {"label":"Bear"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., 1.04, 0.), .68, PALE);
  h = dab(h, p, vec2(0., .84), .27, MID);
  h = eyes(h, p, .3, 1.14, .15);
  h = dab(h, p, vec2(0., .94), .08, DARK);
  h = smile(h, p, vec2(0., .82), .08, .014, DARK);
  vec2 r = h;
  vec2 ear = ball(m, V(.5, 1.6, 0.), .24, PALE);
  r = U(r, dab(ear, m, vec2(.5, 1.6), .13, TONE));
  r = U(r, oval(p, V(0., .3, -.02), V(.44, .34, .36), PALE));
  r = U(r, oval(m, V(.42, .36, .1), V(.13, .2, .14), MID));
  r = U(r, oval(m, V(.24, .08, .16), V(.15, .09, .17), MID));
  return r;
}
