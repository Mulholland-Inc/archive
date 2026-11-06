#pragma frame {"set":["business"],"label":"Clothing boutique","group":"shop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(mirX(p), V(.4,.24,.42), V(.16,.13,.012), .01, MID));
  vec2 q = p.xy - vec2(0,1.2);
  float skirt = max(max(dot(vec2(abs(q.x) - .17, q.y - .06), normalize(vec2(.66,.3))), q.y - .06), -q.y - .54);
  r = U(r, emblem(p, min(skirt, sdRect(q - vec2(0,.26), vec2(.2,.22), .05)), PALE));
  r = U(r, stick(mirX(p), V(0,1.76,0), V(.32,1.56,0), .025, TONE));
  r = U(r, stick(p, V(0,1.76,0), V(0,1.84,0), .025, TONE));
  r = U(r, M(max(sdTorus((p - V(.06,1.86,0)).xzy, .06, .02), 1.84 - p.y), TONE));
  return r;
}
