#pragma frame {"set":["family"],"label":"Pagoda","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  for (int i = 0; i < 3; i++) {
  float y = float(i) * .52, w = .54 - float(i) * .12;
  r = U(r, box(p, V(0,y + .22,0), V(w,.22,w), .01, PALE));
  r = U(r, M(sdCone(p - V(0,y + .52,0), .1, w + .34, w * .55), TONE));
  }
  r = U(r, stick(p, V(0,1.66,0), V(0,1.96,0), .03, TONE));
  return r;
}
