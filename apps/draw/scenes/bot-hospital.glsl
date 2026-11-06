#pragma frame {"set":["business"],"label":"Hospital","group":"health"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.44,0), V(.76,.44,.42), .01, PALE));
  r = U(r, box(p, V(0,.94,0), V(.32,.94,.36), .01, PALE));
  r = U(r, box(p, V(0,1.4,.36), V(.14,.04,.012), .01, TONE));
  r = U(r, box(p, V(0,1.4,.36), V(.04,.14,.012), .01, TONE));
  r = U(r, box(p, V(0,.16,.37), V(.14,.16,.012), .01, DARK));
  r = U(r, box(m, V(.56,.5,.42), V(.12,.1,.012), .01, MID));
  r = U(r, box(m, V(.56,.2,.42), V(.12,.1,.012), .01, MID));
  return r;
}
