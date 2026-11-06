#pragma frame {"set":["business"],"label":"Airport hangar","group":"transport"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(max(max(length(p.xy) - .92, abs(p.z) - .56), -p.y), PALE));
  r = U(r, M(max(max(length(p.xy) - .76, abs(p.z - .56) - .015), -p.y), DARK));
  vec2 q = p.xy - vec2(0,1.3);
  r = U(r, emblem(p, min(min(sdSeg(q, vec2(-.68,.02), vec2(.68,.0)) - .1, sdSeg(q, vec2(.1,.02), vec2(-.3,-.34)) - .07), min(sdSeg(q, vec2(.1,.02), vec2(-.1,.26)) - .05, sdSeg(q, vec2(-.6,.06), vec2(-.74,.34)) - .06)), PALE));
  return r;
}
