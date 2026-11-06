#pragma frame {"set":["personal"],"label":"Campsite","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(sdPrism(p, .78, 1.12, .72), PALE));
  r = U(r, M(sdPrism(p - V(0,0,.74), .3, .54, .03), DARK));
  r = U(r, stick(p, V(0,1.08,.72), V(0,1.36,.72), .018, TONE));
  r = U(r, box(p, V(.1,1.3,.72), V(.1,.06,.01), .01, TONE));
  return r;
}
