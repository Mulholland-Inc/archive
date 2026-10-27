import {
  Activity, Anchor, ArrowLeftRight, BookOpen, Briefcase, Building2, CalendarDays, Car, ChefHat, Cloud, Compass,
  CreditCard, Droplet, Feather, Film, Flame, Flower2, Gem, Gift, GraduationCap, HandCoins, Hexagon, House, IdCard,
  Image, Key, Lamp, Landmark, Leaf, Mail, MapPin, Moon, Mountain, NotebookPen, Package, PawPrint, Pill, Plane,
  Puzzle, Receipt, Repeat, Shapes, ShieldCheck, Sparkles, SquareCheck, Star, Stethoscope, StickyNote, Sun,
  TrendingUp, User, Users, Kanban, Calendar, Table2
} from '@lucide/svelte';

const words = [
  [['account', 'bank'], Landmark],
  [['book', 'books', 'reading'], BookOpen],
  [['event', 'events', 'appointment', 'meeting'], CalendarDays],
  [['gift', 'gifts', 'present'], Gift],
  [['document', 'identity', 'passport', 'id'], IdCard],
  [['insurance', 'policy', 'warranty'], ShieldCheck],
  [['journal', 'diary', 'log'], NotebookPen],
  [['measurement', 'health', 'vital', 'metric'], Activity],
  [['note', 'notes', 'memo'], StickyNote],
  [['payment', 'transaction', 'transfer'], ArrowLeftRight],
  [['card'], CreditCard],
  [['person', 'people', 'contact', 'member'], User],
  [['family', 'group', 'team'], Users],
  [['place', 'location', 'address', 'venue'], MapPin],
  [['possession', 'item', 'thing', 'asset', 'inventory'], Package],
  [['recipe', 'meal', 'food', 'dish'], ChefHat],
  [['subscription', 'renewal', 'plan'], Repeat],
  [['task', 'todo', 'chore'], SquareCheck],
  [['trip', 'travel', 'journey', 'flight'], Plane],
  [['house', 'home', 'property'], House],
  [['car', 'vehicle'], Car],
  [['pet', 'animal'], PawPrint],
  [['company', 'organization', 'organisation', 'employer'], Building2],
  [['invoice', 'receipt', 'bill', 'expense'], Receipt],
  [['email', 'message', 'letter'], Mail],
  [['photo', 'picture', 'image'], Image],
  [['job', 'work', 'project'], Briefcase],
  [['school', 'course', 'education'], GraduationCap],
  [['medication', 'medicine', 'prescription'], Pill],
  [['doctor', 'visit', 'treatment'], Stethoscope],
  [['investment', 'stock', 'portfolio'], TrendingUp],
  [['loan', 'debt', 'mortgage'], HandCoins],
  [['movie', 'film', 'show'], Film]
];

const spare = [Shapes, Hexagon, Star, Leaf, Flame, Feather, Anchor, Compass, Gem, Key, Lamp, Puzzle, Sparkles, Sun, Moon, Cloud, Droplet, Flower2, Mountain];

export const layoutIcons = { table: Table2, board: Kanban, calendar: Calendar };

function hash(s) {
  let h = 0;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return h;
}

export function tableIcons(names) {
  const used = new Set();
  const out = {};
  for (const name of [...names].sort()) {
    const parts = name.split('_').reverse();
    let icon;
    for (const part of parts) {
      const hit = words.find(([ks, ic]) => ks.includes(part) && !used.has(ic));
      if (hit) {
        icon = hit[1];
        break;
      }
    }
    if (!icon) {
      const free = spare.filter((ic) => !used.has(ic));
      icon = free.length ? free[hash(name) % free.length] : Shapes;
    }
    used.add(icon);
    out[name] = icon;
  }
  return out;
}
