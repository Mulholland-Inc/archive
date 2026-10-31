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
  vec3 m = mirX(p);
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.48, 0.0), vec3(0.36, 0.34, 0.28), 0.05), WHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.5, 0.27), vec3(0.28, 0.24, 0.03), 0.02), SKY));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.5, 0.31), vec3(0.11, 0.025, 0.025), 0.02), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.62, 0.3), vec3(0.09, 0.04, 0.01)), WHITE));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.86, 0.0), 0.09, 0.05), GRAPHITE));
  r = U(r, M(sdCapsule(m, vec3(0.38, 0.68, 0.0), vec3(0.46, 0.42, 0.1), 0.05), GRAPHITE));
  r = U(r, M(sdSphere(m - vec3(0.46, 0.4, 0.11), 0.07), WHITE));
  r = U(r, M(sdRoundBox(m - vec3(0.2, 0.08, 0.03), vec3(0.11, 0.08, 0.17), 0.04), GRAPHITE));
  return r;
}
