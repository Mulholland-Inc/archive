#pragma frame {"label":"Ninja"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .98, 0.), .7, TONE);
  h = oblong(h, p, vec2(0., 1.0), vec2(.56, .22), .12, PALE);
  h = eyes(h, p, .27, 1.0, .165);
  vec2 r = h;
  r = U(r, M(sdTorus(p - V(0., 1.32, 0.), .56, .05), MID));
  r = U(r, stick(p, V(-.5, 1.3, -.2), V(-.9, 1.42, -.3), .05, MID));
  r = U(r, stick(p, V(-.5, 1.26, -.2), V(-.86, 1.14, -.3), .05, MID));
  r = U(r, oval(p, V(0., .2, 0.), V(.34, .28, .26), TONE));
  r = U(r, oval(m, V(.38, .28, .1), V(.11, .14, .12), MID));
  return r;
}
