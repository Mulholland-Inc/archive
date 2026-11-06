#pragma frame {"set":["business","family"],"label":"Farm","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,0), V(.62,.42,.5), .02, PALE));
  r = U(r, pitch(p, .84, .74, .44, .56, TONE));
  r = U(r, box(p, V(0,.24,.5), V(.24,.24,.02), .01, DARK));
  r = U(r, stick(p, V(-.22,.04,.52), V(.22,.44,.52), .012, MID));
  r = U(r, stick(p, V(.22,.04,.52), V(-.22,.44,.52), .012, MID));
  r = U(r, box(p, V(0,1.0,.54), V(.09,.07,.02), .02, DARK));
  return r;
}
