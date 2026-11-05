#pragma frame {"set":["business","family"],"label":"School","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.36,0), V(.8,.36,.4), .01, PALE));
  r = U(r, box(p, V(0,.56,0), V(.26,.56,.44), .01, PALE));
  r = U(r, pitch(p, 1.12, .36, .24, .48, TONE));
  r = U(r, canZ(p, V(0,.92,.44), .1, .02, MID));
  r = U(r, stick(p, V(0,1.36,0), V(0,1.74,0), .018, TONE));
  r = U(r, box(p, V(.12,1.66,0), V(.12,.07,.01), .01, TONE));
  r = U(r, box(p, V(0,.14,.44), V(.1,.14,.012), .01, DARK));
  for (int i = 0; i < 2; i++) { r = U(r, box(m, V(.42 + float(i) * .22,.42,.4), V(.07,.13,.012), .01, MID)); }
  return r;
}
