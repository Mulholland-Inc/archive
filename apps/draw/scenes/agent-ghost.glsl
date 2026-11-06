#pragma frame {"label":"Ghost"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  float d = sdCapsule(p, V(0., .3, 0.), V(0., 1.02, 0.), .66);
  float w = .08 + .07 * cos(6. * atan(p.z, p.x));
  vec2 h = M(max(d, w - p.y) * .8, PALE);
  h = eyes(h, p, .27, 1.08, .19);
  h = smile(h, p, vec2(0., .9), .09, .016, TONE);
  h = dab(h, m, vec2(.46, .88), .07, MID);
  vec2 r = h;
  r = U(r, oval(rotZ(m - V(.62, .56, .16), .5), V(0.), V(.11, .19, .11), PALE));
  return r;
}
