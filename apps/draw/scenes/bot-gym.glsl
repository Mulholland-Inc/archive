#pragma frame {"set":["business"],"label":"Gym","group":"care"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .76, .62, .44));
  r = U(r, box(p, V(0,.28,.44), V(.56,.28,.012), .008, MID));
  for (int i = 0; i < 3; i++) { r = U(r, box(p, V(0,.1 + float(i) * .16,.45), V(.56,.01,.012), .006, DARK)); }
  r = U(r, box(mirX(p), V(0.38,0.24,.46), V(.16,.13,.012), .01, MID));
  r = U(r, canX(p, V(0,1.08,0), .05, .54, TONE));
  r = U(r, canX(mirX(p), V(.44,1.08,0), .28, .08, PALE));
  r = U(r, canX(mirX(p), V(.6,1.08,0), .2, .07, PALE));
  return r;
}
