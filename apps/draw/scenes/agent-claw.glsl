#pragma frame {"label":"Claw robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., .92, 0.), V(.54, .44, .42), .15, SHELL);
  h = oblong(h, p, vec2(0., .94), vec2(.41, .23), .11, DARK);
  h = dab(h, m, vec2(.19, .94), .115, EYE);
  vec2 r = h;
  r = U(r, stick(m, V(.55, .8, 0.), V(.82, 1.02, 0.), .06, MID));
  vec3 q = m - V(.86, 1.3, 0.);
  float t = length(vec2(length(q.xy) - .19, q.z)) - .085;
  r = U(r, M(max(t, -max(.075 - abs(q.x), -q.y)), TONE));
  r = U(r, box(p, V(0., .34, 0.), V(.3, .16, .22), .06, SHELL));
  r = U(r, box(m, V(.2, .08, .02), V(.12, .08, .16), .05, TONE));
  return r;
}
