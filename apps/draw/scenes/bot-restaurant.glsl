#pragma frame {"set":["business"],"label":"Restaurant","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .6, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .32, .42));
  vec2 q = p.xy - vec2(-.26,1.24);
  float tines = min(min(sdRect(q - vec2(-.15,.3), vec2(.04,.26), .03), sdRect(q - vec2(.0,.3), vec2(.04,.26), .03)), sdRect(q - vec2(.15,.3), vec2(.04,.26), .03));
  r = U(r, emblem(p, min(min(tines, sdRect(q - vec2(0,.06), vec2(.19,.07), .05)), sdRect(q - vec2(0,-.3), vec2(.055,.36), .04)), PALE));
  vec2 k = p.xy - vec2(.32,1.24);
  r = U(r, emblem(p, min(sdRect(k - vec2(0,-.3), vec2(.055,.36), .04), max(length((k - vec2(-.1,.26)) * vec2(.8,.5)) - .2, .0 - k.x - .03 + .0)), PALE));
  return r;
}
