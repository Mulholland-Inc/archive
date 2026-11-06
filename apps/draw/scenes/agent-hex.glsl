#pragma frame {"label":"Hexagon robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 q = vec2(abs(p.x), p.y - 1.02);
  float d2 = max(q.x * .866 + abs(q.y) * .5, abs(q.y)) - .62;
  vec2 h = M(extrude(d2, p.z, .3) - .04, PALE);
  h = oblong(h, m, vec2(.25, 1.1), vec2(.15, .15), .03, DARK);
  h = oblong(h, m, vec2(.25, 1.1), vec2(.09, .09), .015, EYE);
  h = oblong(h, m, vec2(.06, .8), vec2(.04, .04), .005, TONE);
  h = oblong(h, m, vec2(.18, .84), vec2(.04, .04), .005, TONE);
  vec2 r = h;
  r = U(r, can(p, V(0., .42, 0.), .14, .08, MID));
  r = U(r, box(p, V(0., .22, 0.), V(.38, .14, .26), .05, SHELL));
  r = U(r, canX(m, V(.46, .22, 0.), .1, .06, TONE));
  r = U(r, box(m, V(.2, .05, .04), V(.12, .05, .16), .04, TONE));
  return r;
}
