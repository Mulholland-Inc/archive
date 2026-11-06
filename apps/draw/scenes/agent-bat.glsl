#pragma frame {"label":"Bat"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .96, 0.), .68, PALE);
  h = eyes(h, p, .3, 1.02, .2);
  h = oblong(h, m, vec2(.1, .68), vec2(.035, .05), .01, EYE);
  h = dab(h, p, vec2(0., .84), .04, TONE);
  vec2 r = h;
  vec2 ear = M(sdCone(rotZ(m - V(.4, 1.64, 0.), -.22), .34, .26, .015), PALE);
  r = U(r, ear);
  vec3 q = m - V(.96, .84, -.06);
  float wing = max(sdEllipsoid(q, V(.42, .42, .05)), -sdEllipsoid(q - V(.0, -.5, 0.), V(.2, .28, 1.)));
  wing = max(wing, -sdEllipsoid(q - V(.32, -.36, 0.), V(.15, .2, 1.)));
  wing = max(wing, -sdEllipsoid(q - V(-.32, -.5, 0.), V(.2, .22, 1.)));
  r = U(r, M(wing, TONE));
  r = U(r, oval(p, V(0., .26, 0.), V(.26, .28, .24), TONE));
  return r;
}
