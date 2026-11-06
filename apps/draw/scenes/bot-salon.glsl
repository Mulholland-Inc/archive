#pragma frame {"set":["business"],"label":"Hair salon","group":"care"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec3 c = p - V(0,1.18,0);
  r = U(r, stick(rotZ(c, -.42), V(0,-.44,0), V(0,.54,0), .05, PALE));
  r = U(r, stick(rotZ(c, .42), V(0,-.44,0), V(0,.54,0), .05, PALE));
  vec2 a = rotZ(c, -.42).xy, b = rotZ(c, .42).xy;
  r = U(r, emblem(p - V(0,1.18,0), min(abs(length(a - vec2(0,-.56)) - .16) - .05, abs(length(b - vec2(0,-.56)) - .16) - .05), TONE));
  return r;
}
