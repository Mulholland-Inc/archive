#pragma frame {"label":"Boxing robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., 1.1, 0.), V(.58, .46, .42), .15, SHELL);
  h = oblong(h, p, vec2(0., 1.12), vec2(.45, .2), .09, DARK);
  h = dab(h, m, vec2(.21, 1.12), .105, EYE);
  vec2 r = h;
  r = U(r, box(p, V(0., .46, 0.), V(.3, .22, .24), .08, SHELL));
  r = U(r, stick(m, V(.32, .5, 0.), V(.6, .56, .2), .07, MID));
  r = U(r, oval(m, V(.66, .6, .3), V(.27, .28, .29), TONE));
  r = U(r, box(m, V(.17, .12, .03), V(.13, .12, .17), .05, MID));
  return r;
}
