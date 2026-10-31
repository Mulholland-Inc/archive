#pragma frame {"size":[640,720],"center":[0.0,0.72,0.0],"zoom":270,"bounds":[[-1.0,-0.05,-1.0],[1.0,1.8,1.0]]}

vec2 map(vec3 p) {
  p = rotY(p, 0.5);
  vec3 m = mirX(p);

  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.48, 0.0), vec3(0.36, 0.34, 0.28), 0.05), WHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.5, 0.27), vec3(0.28, 0.24, 0.03), 0.02), SKY));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.5, 0.31), vec3(0.11, 0.025, 0.025), 0.02), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.62, 0.3), vec3(0.09, 0.04, 0.01)), WHITE));

  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.08, 0.0), vec3(0.3, 0.2, 0.24), 0.08), WHITE));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.86, 0.0), 0.09, 0.05) , GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.08, 0.23), vec3(0.23, 0.12, 0.02), 0.03), GRAPHITE));
  r = U(r, M(sdSphere(m - vec3(0.11, 1.09, 0.26), 0.042), LIME));

  r = U(r, M(sdCapsule(p, vec3(0.0, 1.28, 0.0), vec3(0.0, 1.46, 0.0), 0.016), GRAPHITE));
  r = U(r, M(sdSphere(p - vec3(0.0, 1.5, 0.0), 0.055), LIME));

  r = U(r, M(sdCapsule(m, vec3(0.38, 0.68, 0.0), vec3(0.46, 0.42, 0.1), 0.05), GRAPHITE));
  r = U(r, M(sdSphere(m - vec3(0.46, 0.4, 0.11), 0.07), WHITE));
  r = U(r, M(sdRoundBox(m - vec3(0.2, 0.08, 0.03), vec3(0.11, 0.08, 0.17), 0.04), GRAPHITE));
  return r;
}
