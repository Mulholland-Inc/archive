#pragma frame {"set":["personal","family"],"label":"Bungalow","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.32,0), V(.78,.32,.46), .02, PALE));
  r = U(r, pitch(p, .64, .96, .34, .58, TONE));
  r = U(r, box(p, V(.46,.46,.6), V(.02,.3,.02), .01, TONE));
  r = U(r, box(p, V(-.46,.46,.6), V(.02,.3,.02), .01, TONE));
  r = U(r, box(p, V(0,.2,.46), V(.1,.2,.02), .01, DARK));
  return r;
}
