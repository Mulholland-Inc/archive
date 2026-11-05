#pragma frame {"set":["business"],"label":"Mine","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(max(sdEllipsoid(p, V(.92,.92,.76)), -p.y), PALE));
  r = U(r, M(max(sdRoundBox(p - V(0,.22,.72), V(.34,.46,.12), .28), -p.y), TONE));
  r = U(r, M(max(sdRoundBox(p - V(0,.22,.78), V(.25,.36,.12), .24), -p.y), DARK));
  r = U(r, box(p, V(0,.02,.98), V(.24,.02,.16), .01, TONE));
  return r;
}
