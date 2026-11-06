#pragma frame {"label":"Dinosaur"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., 1.0, 0.), V(.74, .6, .56), PALE);
  h = eyes(h, p, .34, 1.16, .17);
  h = smile(h, p, vec2(0., 1.06), .46, .016, DARK);
  vec2 r = h;
  for (int i = 0; i < 3; i++) r = U(r, M(sdCone(rotX(p - V(0., 1.62 - float(i) * .1, .12 - float(i) * .28), -float(i) * .5), .2, .19, .012), TONE));
  r = U(r, oval(p, V(0., .32, -.04), V(.42, .34, .36), PALE));
  r = U(r, stick(m, V(.38, .46, .12), V(.5, .38, .24), .06, MID));
  r = U(r, oval(rotZ(p - V(.5, .28, -.24), -.35), V(0.), V(.42, .13, .13), MID));
  r = U(r, oval(m, V(.22, .07, .1), V(.16, .07, .19), MID));
  return r;
}
