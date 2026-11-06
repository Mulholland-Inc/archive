#pragma frame {"set":["business"],"label":"Hardware store","group":"shop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .7, .58, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .12, .32, .42));
  vec2 q = rotZ(p - V(0,1.22,0), -.55).xy;
  r = U(r, emblem(p, sdRect(q - vec2(0,-.16), vec2(.07,.52), .04), TONE));
  r = U(r, emblem(p, min(sdRect(q - vec2(-.1,.36), vec2(.42,.15), .04), sdRect(q - vec2(.0,.36), vec2(.2,.17), .03)), PALE));
  return r;
}
