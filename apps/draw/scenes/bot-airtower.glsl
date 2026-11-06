#pragma frame {"set":["business"],"label":"Airport tower","group":"transport"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, cone(p, V(0,.7,0), .7, .34, .18, PALE));
  r = U(r, cone(p, V(0,1.42,0), .14, .24, .46, MID));
  r = U(r, can(p, V(0,1.59,0), .52, .03, TONE));
  r = U(r, stick(p, V(0,1.62,0), V(0,1.92,0), .02, TONE));
  r = U(r, ball(p, V(0,1.94,0), .04, TONE));
  r = U(r, box(p, V(0,.1,.32), V(.08,.1,.02), .01, DARK));
  return r;
}
