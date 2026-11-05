#pragma frame {"set":["family"],"label":"Windmill","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, cone(p, V(0,.68,0), .68, .5, .28, PALE));
  r = U(r, cone(p, V(0,1.46,0), .14, .34, .06, TONE));
  vec3 b = p - V(0,1.22,.34);
  for (int i = 0; i < 4; i++) { vec3 q = rotZ(b, float(i) * 1.5708 + .35); r = U(r, box(q, V(.0,.42,0), V(.11,.34,.012), .005, MID)); r = U(r, stick(q, V(0,0,.02), V(0,.74,.02), .016, TONE)); }
  r = U(r, ball(b, V(0,0,.03), .05, TONE));
  r = U(r, box(p, V(0,.14,.48), V(.09,.14,.02), .01, DARK));
  return r;
}
