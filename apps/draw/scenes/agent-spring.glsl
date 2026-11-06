#pragma frame {"label":"Spring robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., 1.18, 0.), V(.6, .46, .42), .2, SHELL);
  h = oblong(h, p, vec2(0., 1.18), vec2(.44, .28), .14, DARK);
  h = oblong(h, m, vec2(.19, 1.18), vec2(.085, .15), .085, EYE);
  vec2 r = h;
  for (int i = 0; i < 4; i++) r = U(r, M(sdTorus(p - V(0., .28 + float(i) * .135, 0.), .22, .07), MID));
  r = U(r, M(sdRoundCylinder(p - V(0., .1, 0.), .44, .11, .06), TONE));
  r = U(r, stick(p, V(0., 1.62, 0.), V(0., 1.74, 0.), .04, TONE));
  return r;
}
