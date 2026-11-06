#pragma frame {"label":"Satellite robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., .9, 0.), V(.6, .46, .44), .17, SHELL);
  h = oblong(h, p, vec2(0., .9), vec2(.45, .27), .13, DARK);
  h = dab(h, m, vec2(.2, .9), .125, EYE);
  vec2 r = h;
  r = U(r, stick(p, V(.0, 1.34, 0.), V(.06, 1.6, 0.), .045, TONE));
  vec3 q = rotZ(rotX(p - V(.1, 1.7, .04), .75), -.35);
  float cup = max(sdSphere(q - V(0., .34, 0.), .54), -(sdSphere(q - V(0., .42, 0.), .54)));
  r = U(r, M(max(cup, q.y - .28), MID));
  r = U(r, stick(q, V(0., .02, 0.), V(0., .3, 0.), .022, TONE));
  r = U(r, ball(q, V(0., .32, 0.), .05, PALE));
  r = U(r, box(p, V(0., .3, 0.), V(.26, .14, .2), .05, SHELL));
  r = U(r, box(m, V(.17, .07, .03), V(.13, .07, .17), .05, TONE));
  return r;
}
