#pragma frame {"set":["business"],"label":"Bridge","group":"transport"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,0), V(.96,.035,.22), .01, PALE));
  r = U(r, box(m, V(.46,.5,0), V(.1,.5,.2), .01, PALE));
  r = U(r, box(m, V(.46,.42,0), V(.1,.42,.205), .01, MID));
  r = U(r, stick(m, V(.46,1.0,0), V(.96,.46,0), .014, TONE));
  r = U(r, stick(m, V(.46,1.0,0), V(0,.5,0), .014, TONE));
  return r;
}
