#pragma frame {"label":"Cog robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 q = p.xy - vec2(0., 1.);
  float a = abs(mod(atan(q.y, q.x) + PI / 8., PI / 4.) - PI / 8.) * length(q);
  float d2 = min(length(q) - .62, max(length(q) - .82, a - .14));
  vec2 h = M(extrude(d2, p.z, .24) - .04, SHELL);
  h = dab(h, p, vec2(0., 1.), .48, DARK);
  h = dab(h, m, vec2(.2, 1.03), .13, EYE);
  vec2 r = h;
  r = U(r, box(p, V(0., .22, 0.), V(.2, .1, .16), .04, MID));
  r = U(r, box(m, V(.16, .05, .03), V(.12, .05, .16), .04, TONE));
  return r;
}
