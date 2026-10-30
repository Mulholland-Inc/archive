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
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.42, 0.0), vec3(0.56, 0.42, 0.46), 0.03), WHITE);
  r = U(r, roof(p, 0.84, 0.66, 0.46, 0.54));
  r = U(r, eyes(p, 0.25, 0.56, 0.46, 0.09));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.17, 0.46), vec3(0.09, 0.17, 0.02), 0.02), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.34, 1.12, -0.14), vec3(0.07, 0.18, 0.07)), STEEL));
  return r;
}
