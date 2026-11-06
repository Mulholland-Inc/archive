#pragma frame {"set":["business"],"label":"Spa","group":"care"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.3,0), V(.72,.3,.46), .05, PALE));
  r = U(r, box(p, V(0,.64,0), V(.78,.04,.52), .03, TONE));
  r = U(r, box(mirX(p), V(0.38,0.24,.46), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .46));
  r = U(r, oval(p, V(0,.84,0), V(.5,.16,.3), PALE));
  r = U(r, oval(p, V(.03,1.12,0), V(.36,.13,.24), MID));
  r = U(r, oval(p, V(-.02,1.34,0), V(.24,.11,.18), PALE));
  r = U(r, oval(p, V(.02,1.52,0), V(.14,.08,.12), MID));
  return r;
}
