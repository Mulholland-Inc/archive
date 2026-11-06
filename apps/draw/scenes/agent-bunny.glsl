#pragma frame {"label":"Bunny"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .86, 0.), V(.64, .56, .52), PALE);
  h = eyes(h, p, .28, .92, .17);
  h = dab(h, p, vec2(0., .76), .05, TONE);
  h = oblong(h, p, vec2(0., .64), vec2(.07, .05), .015, MID);
  h = dab(h, m, vec2(.46, .72), .08, MID);
  vec2 r = h;
  vec2 a = oval(rotZ(p - V(-.26, 1.62, 0.), .12), V(0.), V(.17, .46, .12), PALE);
  a = oblong(a, rotZ(p - V(-.26, 1.62, 0.), .12), vec2(0., 0.), vec2(.07, .32), .07, TONE);
  vec2 b = oval(rotZ(p - V(.36, 1.54, 0.), -.5), V(0.), V(.17, .44, .12), PALE);
  b = oblong(b, rotZ(p - V(.36, 1.54, 0.), -.5), vec2(0., 0.), vec2(.07, .3), .07, TONE);
  r = U(U(r, a), b);
  r = U(r, oval(p, V(0., .26, 0.), V(.34, .28, .28), PALE));
  r = U(r, oval(m, V(.2, .07, .12), V(.14, .07, .17), MID));
  return r;
}
