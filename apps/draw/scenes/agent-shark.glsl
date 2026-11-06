#pragma frame {"label":"Shark"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .82, 0.), V(.72, .78, .58), MID);
  h = p.y < .62 ? vec2(h.x, PALE) : h;
  h = eyes(h, p, .34, 1.04, .15);
  h = oblong(h, p, vec2(0., .58), vec2(.36, .13), .1, DARK);
  float t = abs(fract((p.x + .4) * 5.) - .5) * .18;
  h = p.z > 0. && abs(p.x) < .34 && p.y > .71 - .17 + t ? h : h;
  h = p.z > 0. && abs(p.x) < .32 && abs(p.y - .63) < .08 && p.y > .55 + t + .04 && sdRect(p.xy - vec2(0., .58), vec2(.36, .13), .1) < 0. ? vec2(h.x, PALE) : h;
  vec2 r = h;
  vec3 f = p - V(0., 1.56, -.1);
  r = U(r, M(sdPrism(vec3(f.x * .4 + f.z, f.y, f.x), .3, .36, .06), MID));
  r = U(r, oval(rotZ(m - V(.78, .56, .06), .6), V(0.), V(.09, .28, .2), MID));
  r = U(r, oval(rotY(p - V(0., .06, -.1), 0.), V(0.), V(.38, .06, .3), MID));
  return r;
}
