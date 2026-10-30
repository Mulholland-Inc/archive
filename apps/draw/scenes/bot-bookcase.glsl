#pragma frame {"size":[320,320],"center":[0.0,0.62,0.0],"zoom":118,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,1.9,1.3]]}
#define GLOW GRAPHITE
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
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.7, 0.0), vec3(0.52, 0.7, 0.3), 0.03), WHITE);
  r = U(r, eyes(p, 0.22, 1.08, 0.3, 0.1));
  r = U(r, M(sdBox(p - vec3(0.0, 0.76, 0.31), vec3(0.46, 0.015, 0.01)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(-0.25, 0.44, 0.31), vec3(0.05, 0.27, 0.02)), SKY));
  r = U(r, M(sdBox(p - vec3(-0.1, 0.44, 0.31), vec3(0.06, 0.27, 0.02)), LIME));
  r = U(r, M(sdBox(p - vec3(0.06, 0.44, 0.31), vec3(0.05, 0.27, 0.02)), STEEL));
  r = U(r, M(sdBox(p - vec3(0.24, 0.44, 0.31), vec3(0.08, 0.27, 0.02)), GRAPHITE));
  return r;
}
