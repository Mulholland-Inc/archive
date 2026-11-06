#pragma frame {"set":["business"],"label":"Logistics depot","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.36,-.1), V(.84,.36,.5), .01, PALE));
  r = U(r, box(p, V(0,.76,-.1), V(.88,.04,.54), .01, TONE));
  r = U(r, box(mirX(p), V(.5,.24,.4), V(.22,.24,.012), .01, DARK));
  r = U(r, box(p, V(0,1.18,-.1), V(.42,.38,.34), .02, PALE));
  r = U(r, box(p, V(0,1.18,.24), V(.08,.38,.012), .01, TONE));
  r = U(r, box(p, V(0,1.56,-.1), V(.08,.012,.34), .01, TONE));
  return r;
}
