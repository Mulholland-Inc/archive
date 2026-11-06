#pragma frame {"set":["personal"],"label":"Tipi","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, cone(p, V(0,.68,0), .68, .72, .08, PALE));
  r = U(r, stick(p, V(-.12,1.36,0), V(.14,1.72,.0), .02, TONE));
  r = U(r, stick(p, V(.12,1.36,0), V(-.14,1.72,.0), .02, TONE));
  r = U(r, M(max(sdCone(p - V(0,.68,0), .68, .725, .085), abs(p.y - .38) - .07), TONE));
  r = U(r, M(sdPrism(p - V(0,0,.62), .22, .42, .08), DARK));
  return r;
}
