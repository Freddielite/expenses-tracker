import {
  UtensilsCrossed,
  Car,
  Home,
  Zap,
  Smartphone,
  HeartPulse,
  Clapperboard,
  ShoppingBag,
  Briefcase,
  CircleDollarSign,
  Wallet,
  Gift,
  MoreHorizontal,
  Tag,
} from "lucide-react";

// Maps the icon "key" the backend stores (a plain string) to an actual
// lucide-react component. The backend doesn't know about React or icon
// libraries — it just stores a string like "utensils" and the frontend
// decides what that looks like.
const ICON_MAP = {
  utensils: UtensilsCrossed,
  car: Car,
  home: Home,
  zap: Zap,
  smartphone: Smartphone,
  "heart-pulse": HeartPulse,
  clapperboard: Clapperboard,
  "shopping-bag": ShoppingBag,
  briefcase: Briefcase,
  "circle-dollar-sign": CircleDollarSign,
  wallet: Wallet,
  gift: Gift,
  "more-horizontal": MoreHorizontal,
};

export const ICON_OPTIONS = Object.keys(ICON_MAP);

export function getIconComponent(key) {
  return ICON_MAP[key] || Tag;
}

// Looks up a category's icon + color from the live categories list fetched
// from the API. Falls back to a neutral default if the category was deleted
// or renamed after a transaction referenced it.
export function getCategoryMeta(categories, categoryName) {
  const match = categories.find((c) => c.name === categoryName);
  if (!match) {
    return { icon: Tag, color: "#6b6f76" };
  }
  return { icon: getIconComponent(match.icon), color: match.color };
}
