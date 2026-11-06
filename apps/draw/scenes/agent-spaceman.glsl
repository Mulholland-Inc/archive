#pragma frame {"label":"Astronaut"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., 1.06, 0.), .68, PALE);
  h = dab(h, p, vec2(0., 1.06), .5, DARK);
  h = dab(h, m, vec2(.2, 1.08), .13, EYE);
  h = line(h, p, vec2(.28, 1.38), vec2(.38, 1.28), .018, EYE);
  vec2 r = h;
  r = U(r, canX(m, V(.7, 1.06, 0.), .14, .05, TONE));
  r = U(r, stick(p, V(.24, 1.66, 0.), V(.32, 1.86, 0.), .025, TONE));
  r = U(r, box(p, V(0., .3, 0.), V(.38, .3, .28), .12, SHELL));
  r = oblong(r, p, vec2(0., .32), vec2(.2, .1), .03, TONE);
  r = U(r, box(p, V(0., .36, -.32), V(.3, .26, .1), .05, MID));
  r = U(r, stick(m, V(.4, .44, 0.), V(.54, .24, .06), .08, PALE));
  return r;
}
