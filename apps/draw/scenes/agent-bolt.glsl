#pragma frame {"label":"Boxy robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., 1.02, 0.), V(.62, .5, .46), .13, SHELL);
  h = oblong(h, p, vec2(0., 1.02), vec2(.47, .28), .11, DARK);
  h = dab(h, m, vec2(.21, 1.02), .135, EYE);
  vec2 r = h;
  r = U(r, box(m, V(.67, 1.02, 0.), V(.07, .17, .17), .04, MID));
  r = U(r, stick(p, V(0., 1.5, 0.), V(0., 1.7, 0.), .035, TONE));
  r = U(r, ball(p, V(0., 1.78, 0.), .11, PALE));
  r = U(r, box(p, V(0., .36, 0.), V(.36, .2, .26), .07, SHELL));
  r = U(r, stick(m, V(.42, .44, 0.), V(.5, .24, .04), .075, MID));
  r = U(r, box(m, V(.19, .08, .03), V(.13, .08, .18), .05, TONE));
  return r;
}
