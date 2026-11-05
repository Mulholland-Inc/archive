#pragma frame {"label":"Diving robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .98, 0.), .7, PALE);
  h = dab(h, p, vec2(0., .98), .49, TONE);
  h = dab(h, p, vec2(0., .98), .39, DARK);
  h = dab(h, m, vec2(.17, 1.), .12, EYE);
  vec2 a = vec2(p.x, p.y - .98);
  float t = atan(a.y, a.x), u = mod(t + PI / 8., PI / 4.) - PI / 8.;
  h = dab(h, vec3(length(a) * cos(u), length(a) * sin(u), p.z), vec2(.44, 0.), .028, PALE);
  vec2 r = h;
  r = U(r, M(sdRoundCylinder(p - V(0., .3, 0.), .46, .12, .06), TONE));
  r = U(r, stick(p, V(.1, 1.64, -.06), V(.22, 1.8, -.2), .05, MID));
  r = U(r, box(m, V(.24, .08, .05), V(.17, .08, .23), .06, MID));
  return r;
}
