#pragma frame {"set":["business"],"label":"Architecture studio","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(mirX(p), V(.4,.24,.42), V(.16,.13,.012), .01, MID));
  vec2 q = p.xy - vec2(-.5,.72);
  vec2 n = normalize(vec2(1.,.95));
  r = U(r, emblem(p, max(max(max(-q.x, -q.y), dot(q - vec2(0.,1.16), n)), -max(max(.22 - q.x, .22 - q.y), dot(q - vec2(0.,.72), n))), PALE));
  r = U(r, stick(p, V(.08,1.62,.1), V(.46,1.0,.1), .05, TONE));
  return r;
}
