#pragma frame {"set":["business"],"label":"Town hall","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.34,0), V(.82,.34,.42), .01, PALE));
  r = U(r, box(p, V(0,.72,0), V(.86,.04,.46), .01, TONE));
  for (int i = 0; i < 2; i++) { r = U(r, box(m, V(.42 + float(i) * .22,.34,.42), V(.07,.2,.012), .01, MID)); }
  r = U(r, door(p, 0., .09, .24, .42));
  r = U(r, box(p, V(0,1.14,0), V(.3,.38,.3), .01, PALE));
  r = U(r, canZ(p, V(0,1.2,.3), .25, .03, MID));
  r = U(r, stick(p, V(0,1.2,.34), V(0,1.38,.34), .016, DARK));
  r = U(r, stick(p, V(0,1.2,.34), V(.13,1.14,.34), .016, DARK));
  r = U(r, cone(p, V(0,1.74,0), .22, .38, .02, TONE));
  return r;
}
