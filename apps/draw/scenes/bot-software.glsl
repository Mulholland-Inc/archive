#pragma frame {"set":["business"],"label":"Software company","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.46,0), V(.6,.46,.46), .05, PALE));
  r = U(r, box(p, V(0,.34,.46), V(.5,.012,.012), .006, MID));
  r = U(r, box(mirX(p), V(0.38,0.24,.46), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .28, .46));
  r = U(r, box(p, V(0,1.42,0), V(.7,.42,.07), .05, PALE));
  r = U(r, box(p, V(0,1.42,.07), V(.6,.33,.015), .03, TONE));
  r = U(r, can(p, V(0,.96,0), .1, .06, TONE));
  vec2 q = p.xy - vec2(0,1.42);
  r = U(r, M(extrude(min(min(min(sdSeg(vec2(abs(q.x), q.y), vec2(.42,0.), vec2(.24,.18)), sdSeg(vec2(abs(q.x), q.y), vec2(.42,0.), vec2(.24,-.18))) - .04, 1e3), sdSeg(q, vec2(-.08,-.2), vec2(.08,.2)) - .035), p.z - .08, .02), DARK));
  return r;
}
