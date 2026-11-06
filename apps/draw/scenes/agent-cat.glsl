#pragma frame {"label":"Cat"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., 1., 0.), V(.74, .62, .54), PALE);
  h = eyes(h, p, .32, 1.04, .185);
  h = dab(h, p, vec2(0., .84), .045, TONE);
  h = line(h, m, vec2(.54, .88), vec2(.82, .94), .012, TONE);
  h = line(h, m, vec2(.54, .78), vec2(.82, .76), .012, TONE);
  vec2 r = h;
  r = U(r, M(sdCone(rotZ(m - V(.46, 1.58, 0.), -.3), .25, .2, .015), PALE));
  r = U(r, oval(p, V(0., .3, -.02), V(.42, .34, .34), PALE));
  r = U(r, oval(m, V(.2, .08, .14), V(.13, .08, .15), MID));
  r = U(r, stick(p, V(.3, .2, -.25), V(.66, .5, -.28), .07, TONE));
  return r;
}
