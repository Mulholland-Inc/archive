#pragma frame {"size":[320,320],"center":[0.0,0.72,0.0],"zoom":128,"pitch":20,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,2.0,1.3]]}
#define GLOW GRAPHITE
float gable(vec3 q, float h, float w) {
  return (h * abs(q.x) + w * q.y - h * w) / length(vec2(h, w));
}
vec2 roof(vec3 p, float y, float w, float h, float d) {
  vec3 g = p - vec3(0.0, y, 0.0);
  return M(max(max(gable(g, h, w), -g.y), abs(g.z) - d), WHITE);
}
vec2 eyes(vec3 p, float x, float y, float z) {
  vec3 m = mirX(p);
  return M(sdRoundBox(m - vec3(x, y, z), vec3(0.075, 0.125, 0.025), 0.06), GLOW + SOLID);
}
float tooth(vec3 q) {
  vec2 n = normalize(vec2(-0.38, 0.5));
  return max(max(max(-q.y, q.x - 0.25), dot(q.xy - vec2(-0.25, 0.0), n)), abs(q.z) - 0.5);
}
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.42, 0.0), vec3(0.62, 0.42, 0.5), 0.04), WHITE + SOLID);
  r = U(r, roof(p, 0.84, 0.76, 0.56, 0.56));
  r = U(r, eyes(p, 0.22, 0.56, 0.51));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.17, 0.51), vec3(0.2, 0.17, 0.015), 0.02), GRAPHITE));
  return r;
}
