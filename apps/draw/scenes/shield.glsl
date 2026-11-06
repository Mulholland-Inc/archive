#pragma frame {"size":[500,500],"center":[0.0,0.0,0.0],"zoom":210,"bounds":[[-1.0,-1.1,-1.0],[1.0,1.0,1.0]]}

float crest(vec2 q) {
  float x = abs(q.x);
  return max(max(length(vec2(x + 0.7, q.y - 0.2)) - 1.2, q.y - 0.6), x - 0.5);
}

vec2 map(vec3 p) {
  vec3 q = rotY(p, 0.5);
  float d = crest(q.xy);
  vec2 r = M(extrude(d, q.z, 0.05), WHITE);
  r = U(r, M(extrude(d + 0.09, q.z - 0.07, 0.025), LIME));
  float tick = min(sdSeg(q.xy, vec2(-0.2, 0.0), vec2(-0.05, -0.17)), sdSeg(q.xy, vec2(-0.05, -0.17), vec2(0.22, 0.2))) - 0.055;
  return U(r, M(extrude(tick, q.z - 0.105, 0.015), WHITE));
}
