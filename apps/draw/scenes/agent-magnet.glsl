#pragma frame {"label":"Magnet robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 q = p.xy - vec2(0., .98);
  float d2 = q.y > 0. ? abs(length(q) - .54) - .3 : max(abs(abs(q.x) - .54) - .3, -q.y - .96);
  vec2 h = M(extrude(d2, p.z, .28) - .06, SHELL);
  h = p.y < .3 ? vec2(h.x, TONE) : h;
  h = eyes(h, p, .27, 1.44, .2);
  vec2 r = h;
  r = U(r, stick(p, V(0., 1.86, 0.), V(0., 1.98, 0.), .035, TONE));
  r = U(r, ball(p, V(0., 2.02, 0.), .07, PALE));
  return r;
}
