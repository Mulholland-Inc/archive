#pragma frame {"size":[320,320],"center":[0.0,0.62,0.0],"zoom":118,"trim":false,"bounds":[[-1.3,-0.1,-1.3],[1.3,1.9,1.3]]}
#define GLOW WHITE
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
  vec2 r = M(sdRoundBox(p - vec3(0.0, 1.08, 0.0), vec3(0.3, 0.2, 0.24), 0.08), WHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.08, 0.23), vec3(0.23, 0.12, 0.02), 0.03), GRAPHITE));
  r = U(r, M(sdSphere(m - vec3(0.11, 1.09, 0.26), 0.042), GLOW));
  r = U(r, M(sdCapsule(p, vec3(0.0, 1.28, 0.0), vec3(0.0, 1.46, 0.0), 0.016), GRAPHITE));
  r = U(r, M(sdSphere(p - vec3(0.0, 1.5, 0.0), 0.055), GLOW));
  return r;
}
