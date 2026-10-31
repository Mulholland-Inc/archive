#pragma frame {"size":[320,320],"center":[0.0,1.42,0.0],"zoom":138,"pitch":12,"trim":false,"bounds":[[-1.1,0.2,-1.1],[1.1,2.7,1.1]]}
vec2 map(vec3 p) {
  p = rotY(p, 0.785);
  vec3 m = mirX(p);
  // Head, ears and nose.
  vec2 r = M(sdRoundBox(p - vec3(0.0, 1.1, 0.0), vec3(0.4, 0.42, 0.34), 0.17), WHITE);
  r = U(r, M(sdSphere(m - vec3(0.44, 1.12, 0.0), 0.09), WHITE));
  r = U(r, M(sdSphere(p - vec3(0.0, 1.06, 0.37), 0.075), WHITE));
  // Eyes, and a monocle on one of them with its chain.
  r = U(r, M(sdSphere(m - vec3(0.17, 1.22, 0.32), 0.05), GRAPHITE));
  r = U(r, M(sdTorus((p - vec3(-0.17, 1.22, 0.34)).xzy, 0.125, 0.018), STEEL));
  r = U(r, M(sdCapsule(p, vec3(-0.29, 1.17, 0.33), vec3(-0.43, 0.72, 0.32), 0.009), STEEL));
  // A moustache that curls up at the ends.
  r = U(r, M(sdCapsule(m, vec3(0.02, 0.97, 0.36), vec3(0.17, 0.96, 0.33), 0.04), GRAPHITE));
  r = U(r, M(sdCapsule(m, vec3(0.17, 0.96, 0.33), vec3(0.25, 1.02, 0.3), 0.024), GRAPHITE));
  // A hat too tall for the job, slightly crooked.
  vec3 h = rotZ(p - vec3(0.0, 1.5, 0.0), -0.14);
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.03, 0.0), 0.56, 0.03), GRAPHITE));
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.4, 0.0), 0.33, 0.36), GRAPHITE));
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.15, 0.0), 0.34, 0.09), SKY));
  // Collar and bow tie.
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.66, 0.0), 0.12, 0.06), WHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.38, 0.0), vec3(0.44, 0.22, 0.3), 0.08), GRAPHITE));
  r = U(r, M(sdSphere(p - vec3(0.0, 0.58, 0.33), 0.04), SKY));
  r = U(r, M(sdRoundBox(m - vec3(0.09, 0.58, 0.31), vec3(0.07, 0.05, 0.02), 0.015), SKY));
  return r;
}
