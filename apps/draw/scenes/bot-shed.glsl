#pragma frame {"set":["personal"],"label":"Garden shed","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.4,0), V(.56,.4,.42), .01, PALE));
  r = U(r, M(max(dot(p.xy - vec2(0.,1.), normalize(vec2(.26,1.))), max(.8 - p.y, max(abs(p.x) - .66, abs(p.z) - .52))), TONE));
  r = U(r, box(p, V(-.26,.3,.42), V(.13,.3,.02), .01, DARK));
  r = U(r, box(p, V(.26,.42,.42), V(.16,.16,.012), .01, MID));
  return r;
}
