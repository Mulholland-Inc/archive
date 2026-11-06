#pragma frame {"set":["family"],"label":"Lighthouse","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, cone(p, V(0,.72,0), .72, .46, .26, PALE));
  r = U(r, M(max(cone(p, V(0,.72,0), .72, .47, .27, TONE).x, abs(p.y - .84) - .16), TONE));
  r = U(r, can(p, V(0,1.46,0), .34, .03, MID));
  r = U(r, can(p, V(0,1.62,0), .2, .14, DARK));
  r = U(r, cone(p, V(0,1.86,0), .12, .26, .02, PALE));
  r = U(r, box(p, V(0,.14,.43), V(.09,.14,.03), .02, DARK));
  r = U(r, can(p, V(0,.03,0), .62, .03, PALE));
  return r;
}
