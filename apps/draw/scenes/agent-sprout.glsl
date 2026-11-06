#pragma frame {"label":"Sprout"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .68, 0.), V(.66, .62, .54), PALE);
  h = eyes(h, p, .27, .74, .18);
  h = smile(h, p, vec2(0., .58), .09, .016, TONE);
  h = dab(h, m, vec2(.46, .56), .08, MID);
  vec2 r = h;
  r = U(r, stick(p, V(0., 1.26, 0.), V(0., 1.56, 0.), .035, TONE));
  r = U(r, oval(rotZ(m - V(.27, 1.68, 0.), .95), V(0.), V(.13, .32, .06), TONE));
  r = U(r, oval(m, V(.2, .07, .1), V(.13, .07, .15), MID));
  r = U(r, oval(m, V(.66, .6, 0.), V(.08, .15, .11), MID));
  return r;
}
