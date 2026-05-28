export const DIVISIONS = {
  0: { name: 'Open', color: '#808080', min: 0, max: 9999 },
  1: { name: 'Div. 1', color: '#FF0000', min: 1900, max: 9999 },
  2: { name: 'Div. 2', color: '#0000FF', min: 0, max: 2099 },
  3: { name: 'Div. 3', color: '#008000', min: 0, max: 1599 },
  4: { name: 'Div. 4', color: '#808080', min: 0, max: 1399 },
} as const;

export type Division = keyof typeof DIVISIONS;

export function getDivisionInfo(division: Division) {
  return DIVISIONS[division] || DIVISIONS[0];
}

export function getDivisionName(division: Division): string {
  return getDivisionInfo(division).name;
}

export function getDivisionColor(division: Division): string {
  return getDivisionInfo(division).color;
}
