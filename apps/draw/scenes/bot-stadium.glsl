#pragma frame {"set":["business"],"label":"Stadium","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  vec2 e = p.xz * vec2(1., 1.2);
  r = U(r, M(max(length(e) - .96 - p.y * .1, abs(p.y - .3) - .3), PALE));
  r = U(r, M(max(max(length(e) - .72, .46 - p.y), p.y - .61), DARK));
  r = U(r, M(max(max(length(e) - .56, .5 - p.y), p.y - .62), MID));
  r = U(r, M(max(length(e) - 1.01, abs(p.y - .14) - .05), TONE));
  r = U(r, door(p, 0., .1, .22, .84));
  r = U(r, stick(mirZ(m), V(.74,.3,.42), V(.84,1.14,.48), .03, TONE));
  r = U(r, box(mirZ(m), V(.84,1.2,.48), V(.14,.07,.02), .015, PALE));
  return r;
}
