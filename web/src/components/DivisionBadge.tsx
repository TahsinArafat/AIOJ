import { getDivisionName, getDivisionColor, type Division } from '../lib/divisions';

interface DivisionBadgeProps {
  division: Division;
  size?: 'sm' | 'md' | 'lg';
}

export default function DivisionBadge({ division, size = 'md' }: DivisionBadgeProps) {
  const name = getDivisionName(division);
  const color = getDivisionColor(division);

  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };

  if (division === 0) {
    return (
      <span className={`inline-flex items-center font-medium rounded bg-gray-100 text-gray-600 ${sizeClasses[size]}`}>
        {name}
      </span>
    );
  }

  return (
    <span
      className={`inline-flex items-center font-medium rounded ${sizeClasses[size]}`}
      style={{ color, backgroundColor: `${color}20` }}
    >
      {name}
    </span>
  );
}
