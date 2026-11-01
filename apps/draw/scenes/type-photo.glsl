#pragma frame {"size":[256,256],"center":[0.0,0.55,0.0],"zoom":104,"pitch":30,"trim":false,"bounds":[[-1.4,-0.1,-1.4],[1.4,1.5,1.4]]}
vec2 map(vec3 p) {
  vec3 q = rotX(p - vec3(0.0, 0.55, 0.0), -0.15);
  vec2 r = M(sdBox(q, vec3(0.62, 0.52, 0.05)), GRAPHITE + SOLID);
  r = Sc(r, M(max(sdRect(q.xy, vec2(0.5, 0.4), 0.0), -(q.z - 0.0)), SKY));
  vec2 hill = M(max(max(sdRect(q.xy, vec2(0.5, 0.4), 0.0), length(q.xy - vec2(0.15, -0.75)) - 0.6), abs(q.z) - 0.02), BLUE + SOLID);
  r = U(r, hill);
  r = U(r, M(max(length(q.xy - vec2(-0.25, 0.18)) - 0.09, abs(q.z) - 0.02), WHITE + SOLID));
  r = U(r, M(sdBox(rotX(p - vec3(0.0, 0.3, -0.25), 0.5), vec3(0.06, 0.3, 0.03)), GRAPHITE));
  return r;
}
