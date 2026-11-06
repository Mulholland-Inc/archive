#pragma frame {"set":["business"],"label":"Real estate agency","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = rotZ(p - V(0,1.22,0), .78).xy;
  float k = min(abs(length(q - vec2(0,.28)) - .2) - .08, min(sdRect(q - vec2(0,-.16), vec2(.06,.36), .01), min(sdRect(q - vec2(.12,-.42), vec2(.1,.05), .01), sdRect(q - vec2(.1,-.24), vec2(.08,.05), .01))));
  r = U(r, emblem(p, k, PALE));
  return r;
}
