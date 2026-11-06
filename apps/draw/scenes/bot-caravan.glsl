#pragma frame {"set":["personal"],"label":"Caravan","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.64,0), V(.34,.46,.8), .22, PALE));
  r = U(r, box(p, V(0,.32,0), V(.345,.08,.805), .02, MID));
  r = U(r, canX(m, V(.34,.2,-.1), .19, .05, TONE));
  r = U(r, box(p, V(.35,.7,.3), V(.012,.16,.24), .03, DARK));
  r = U(r, stick(p, V(0,.24,.8), V(0,.2,1.06), .03, TONE));
  return r;
}
