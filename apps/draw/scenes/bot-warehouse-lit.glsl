#pragma frame {"size":[320,320],"center":[0.0,0.62,0.0],"zoom":118,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,1.9,1.3]]}
#define GLOW LIME
float gable(vec3 q, float h, float w) {
  return (h * abs(q.x) + w * q.y - h * w) / length(vec2(h, w));
}
vec2 roof(vec3 p, float y, float w, float h, float d) {
  vec3 g = p - vec3(0.0, y, 0.0);
  return M(max(max(gable(g, h, w), -g.y), abs(g.z) - d), GRAPHITE);
}
vec2 eyes(vec3 p, float x, float y, float z, float r) {
  vec3 m = mirX(p);
  return M(sdRoundBox(m - vec3(x, y, z), vec3(r, r, 0.02), 0.02), GLOW);
}
vec2 map(vec3 p) {
  p = rotY(p, 0.5);
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.42, 0.0), vec3(0.8, 0.42, 0.58), 0.03), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.88, 0.0), vec3(0.84, 0.04, 0.62)), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.3, 0.58), vec3(0.3, 0.3, 0.02), 0.01), STEEL));
  r = U(r, eyes(p, 0.52, 0.62, 0.58, 0.08));
  return r;
}
