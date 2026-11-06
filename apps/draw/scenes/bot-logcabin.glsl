#pragma frame {"set":["personal","family"],"label":"Log cabin","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  for (int i = 0; i < 5; i++) { r = U(r, canX(p, V(0,.09 + float(i) * .155,.5), .08, .7, PALE)); r = U(r, canX(p, V(0,.09 + float(i) * .155,-.5), .08, .7, PALE)); r = U(r, canZ(m, V(.6,.09 + float(i) * .155,0), .08, .6, PALE)); }
  r = U(r, box(p, V(0,.4,0), V(.62,.38,.5), .01, PALE));
  r = U(r, pitch(p, .78, .84, .44, .68, TONE));
  r = U(r, box(p, V(.42,1.14,0), V(.08,.2,.08), .01, DARK));
  r = U(r, box(p, V(0,.16,.57), V(.09,.16,.015), .01, DARK));
  return r;
}
