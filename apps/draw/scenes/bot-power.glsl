#pragma frame {"set":["business"],"label":"Power station","group":"energy"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(max(length(p.xz) - (.34 + .3 * pow(p.y - 1.05, 2.)), abs(p.y - .7) - .7) * .7, PALE));
  r = U(r, can(p, V(0,1.41,0), .3, .012, DARK));
  r = U(r, can(p, V(0,.04,0), .78, .04, TONE));
  r = U(r, oval(p, V(0,1.62,0), V(.3,.17,.3), MID));
  r = U(r, oval(p, V(.2,1.84,0), V(.22,.13,.22), MID));
  return r;
}
