#pragma frame {"set":["business"],"label":"Cinema","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,0), V(.7,.42,.42), .01, PALE));
  r = U(r, box(p, V(0,.72,.46), V(.5,.1,.04), .01, TONE));
  r = U(r, box(p, V(0,.88,0), V(.74,.04,.46), .01, TONE));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .13, .3, .42));
  r = U(r, canZ(p, V(-.2,1.32,0), .4, .07, PALE));
  for (int i = 0; i < 5; i++) { float a = float(i) * 1.257 + .3; r = U(r, canZ(p, V(-.2 + cos(a) * .22,1.32 + sin(a) * .22,.06), .08, .02, DARK)); }
  r = U(r, canZ(p, V(.42,1.14,0), .22, .07, PALE));
  for (int i = 0; i < 4; i++) { float a = float(i) * 1.57 + .5; r = U(r, canZ(p, V(.42 + cos(a) * .12,1.14 + sin(a) * .12,.06), .045, .02, DARK)); }
  return r;
}
