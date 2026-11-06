#pragma frame {"label":"Rotor robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .88, 0.), V(.7, .58, .58), PALE);
  h = oblong(h, p, vec2(0., .92), vec2(.48, .22), .2, DARK);
  h = dab(h, m, vec2(.22, .92), .13, EYE);
  vec2 r = h;
  r = U(r, stick(p, V(0., 1.42, 0.), V(0., 1.62, 0.), .045, TONE));
  r = U(r, oval(rotY(p - V(0., 1.66, 0.), .5), V(0.), V(.92, .04, .12), MID));
  r = U(r, ball(p, V(0., 1.68, 0.), .08, PALE));
  r = U(r, stick(m, V(.26, .38, 0.), V(.34, .1, 0.), .035, TONE));
  r = U(r, stick(m, V(.34, .08, -.3), V(.34, .08, .32), .045, MID));
  return r;
}
