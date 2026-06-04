// web/src/lib/rating.ts

export interface RatingColor {
  name: string;
  hex: string;
  bg: string;
  text: string;
  // Dark-mode variants — adjusted for visibility on gray-800 (#1f2937) background.
  // Text is a lighter shade; bg is the same hue at ~25% opacity for better
  // contrast on dark surfaces than the 20% used in light mode.
  darkBg?: string;
  darkText?: string;
}

export function getRatingColor(rating: number): RatingColor {
  if (rating >= 2900) {
    return { name: 'legendary-grandmaster', hex: '#FF0000', bg: '#FF000020', text: '#FF0000', darkBg: '#FF000040', darkText: '#FCA5A5' };
  } else if (rating >= 2600) {
    return { name: 'international-grandmaster', hex: '#FF0000', bg: '#FF000020', text: '#FF0000', darkBg: '#FF000040', darkText: '#FCA5A5' };
  } else if (rating >= 2400) {
    return { name: 'grandmaster', hex: '#FF8C00', bg: '#FF8C0020', text: '#FF8C00', darkBg: '#FF8C0040', darkText: '#FDBA74' };
  } else if (rating >= 2300) {
    return { name: 'international-master', hex: '#FF8C00', bg: '#FF8C0020', text: '#FF8C00', darkBg: '#FF8C0040', darkText: '#FDBA74' };
  } else if (rating >= 2100) {
    return { name: 'master', hex: '#FFD700', bg: '#FFD70020', text: '#B8860B', darkBg: '#FFD70040', darkText: '#FCD34D' };
  } else if (rating >= 1900) {
    return { name: 'candidate-master', hex: '#AA00AA', bg: '#AA00AA20', text: '#AA00AA', darkBg: '#AA00AA40', darkText: '#D8B4FE' };
  } else if (rating >= 1600) {
    return { name: 'expert', hex: '#0000FF', bg: '#0000FF20', text: '#0000FF', darkBg: '#3B82F640', darkText: '#93C5FD' };
  } else if (rating >= 1400) {
    return { name: 'specialist', hex: '#03A89E', bg: '#03A89E20', text: '#03A89E', darkBg: '#03A89E40', darkText: '#5EEAD4' };
  } else if (rating >= 1200) {
    return { name: 'pupil', hex: '#008000', bg: '#00800020', text: '#008000', darkBg: '#22C55E40', darkText: '#86EFAC' };
  } else {
    return { name: 'newbie', hex: '#808080', bg: '#80808020', text: '#808080', darkBg: '#9CA3AF30', darkText: '#D1D5DB' };
  }
}

export function getRatingTitle(rating: number): string {
  const color = getRatingColor(rating);
  return color.name.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
}
