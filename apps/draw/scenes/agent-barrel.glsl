#pragma frame {"label":"Barrel robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = M(sdRoundCylinder(p - V(0., .98, 0.), .6, .72, .12), PALE);
  h = abs(p.y - 1.46) < .05 || abs(p.y - .5) < .05 ? vec2(h.x, TONE) : h;
  h = oblong(h, p, vec2(0., 1.04), vec2(.42, .2), .1, DARK);
  h = dab(h, m, vec2(.2, 1.04), .115, EYE);
  vec2 r = h;
  r = U(r, ringZ(m, V(.66, 1.04, 0.), .13, .035, MID));
  r = U(r, box(m, V(.3, .14, .02), V(.15, .14, .2), .05, TONE));
  return r;
}
