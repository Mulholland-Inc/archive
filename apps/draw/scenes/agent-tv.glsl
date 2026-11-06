#pragma frame {"label":"Television robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., .95, 0.), V(.72, .54, .4), .1, SHELL);
  h = oblong(h, p, vec2(-.1, .95), vec2(.48, .4), .14, DARK);
  h = dab(h, vec3(p.x + .1, p.y, p.z) * vec3(sign(p.x + .1), 1., 1.), vec2(.2, .98), .13, EYE);
  h = dab(h, p, vec2(.6, 1.14), .05, TONE);
  h = dab(h, p, vec2(.6, .96), .05, TONE);
  vec2 r = h;
  r = U(r, stick(m, V(.08, 1.46, 0.), V(.42, 1.86, 0.), .028, TONE));
  r = U(r, ball(m, V(.43, 1.88, 0.), .06, PALE));
  r = U(r, box(m, V(.34, .22, 0.), V(.09, .22, .1), .04, MID));
  r = U(r, box(m, V(.36, .06, .05), V(.17, .06, .17), .05, TONE));
  return r;
}
