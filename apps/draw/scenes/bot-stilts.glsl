#pragma frame {"set":["personal","family"],"label":"Stilt house","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.84,0), V(.52,.28,.4), .01, PALE));
  r = U(r, pitch(p, 1.12, .66, .36, .5, TONE));
  r = U(r, stick(mirZ(m), V(.42,0,.32), V(.42,.56,.32), .04, TONE));
  r = U(r, box(p, V(0,.55,.06), V(.62,.03,.5), .01, TONE));
  for (int i = 0; i < 4; i++) { r = U(r, box(p, V(0,.42 - float(i) * .13,.62 + float(i) * .09), V(.12,.012,.04), .006, MID)); }
  return r;
}
