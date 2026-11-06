#pragma frame {"set":["business","family"],"label":"Greenhouse","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.34,0), V(.62,.34,.74), .01, PALE));
  r = U(r, M(max(sdCapsule(p, V(0,.68,-.74), V(0,.68,.74), .62), .68 - p.y), MID));
  r = U(r, box(p, V(0,.5,.74), V(.012,.5,.012), .006, TONE));
  for (int i = 0; i < 4; i++) { r = U(r, box(p, V(.63,.5,-.56 + float(i) * .37), V(.012,.5,.012), .006, TONE)); }
  r = U(r, box(p, V(0,.15,.74), V(.09,.15,.012), .01, DARK));
  return r;
}
