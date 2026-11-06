#pragma frame {"label":"Snail"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec3 c = p - V(0., 1.02, -.2);
  float sp = length(c.xy) - (atan(c.y, c.x) + PI) / TAU * .24;
  vec2 s = ball(c, V(0.), .66, PALE);
  s = c.z > 0. ? paint(s, abs(mod(sp, .24) - .12) - .022, TONE) : s;
  vec2 h = ball(p, V(0., .5, .44), .42, MID);
  vec3 g = p - V(0., 0., .44);
  h = eyes(h, vec3(p.x, p.y, g.z), .19, .56, .13);
  h = smile(h, vec3(p.x, p.y, g.z), vec2(0., .42), .07, .014, DARK);
  vec2 r = U(s, h);
  r = U(r, M(sdEllipsoid(p - V(0., .1, .1), V(.62, .14, .66)), MID));
  r = U(r, stick(m, V(.14, .86, .44), V(.22, 1.1, .5), .03, MID));
  r = U(r, ball(m, V(.22, 1.14, .5), .06, TONE));
  return r;
}
