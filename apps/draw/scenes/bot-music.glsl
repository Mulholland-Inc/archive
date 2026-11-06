#pragma frame {"set":["business"],"label":"Music studio","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(mirX(p), V(.4,.24,.42), V(.16,.13,.012), .01, MID));
  vec2 q = p.xy - vec2(0,.9);
  r = U(r, emblem(p, min(min(sdCircle(q - vec2(-.3,0.), .19), sdCircle(q - vec2(.3,.06), .19)), min(min(sdSeg(q, vec2(-.13,.0), vec2(-.13,.74)) - .05, sdSeg(q, vec2(.47,.06), vec2(.47,.8)) - .05), sdSeg(q, vec2(-.13,.74), vec2(.47,.8)) - .09)), PALE));
  return r;
}
