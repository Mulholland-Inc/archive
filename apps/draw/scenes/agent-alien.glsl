#pragma frame {"label":"Alien"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  float f = smin(sdEllipsoid(p - V(0., 1.08, 0.), V(.72, .54, .54)), sdEllipsoid(p - V(0., .68, 0.), V(.3, .3, .28)), .3);
  vec2 h = M(f, PALE);
  h = eyes(h, p, .3, 1.02, .23);
  h = smile(h, p, vec2(0., .68), .08, .014, TONE);
  vec2 r = h;
  r = U(r, stick(m, V(.28, 1.5, 0.), V(.42, 1.78, 0.), .028, TONE));
  r = U(r, ball(m, V(.43, 1.82, 0.), .08, PALE));
  r = U(r, oval(p, V(0., .2, 0.), V(.22, .26, .18), MID));
  r = U(r, stick(m, V(.18, .3, 0.), V(.4, .14, .04), .04, MID));
  return r;
}
