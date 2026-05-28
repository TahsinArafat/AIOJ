// web/src/components/RatingBadge.tsx
import { getRatingColor, getRatingTitle } from '../lib/rating';

interface RatingBadgeProps {
  rating: number;
  showTitle?: boolean;
  size?: 'sm' | 'md' | 'lg';
}

export default function RatingBadge({ rating, showTitle = false, size = 'md' }: RatingBadgeProps) {
  const color = getRatingColor(rating);
  const title = getRatingTitle(rating);

  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };

  return (
    <span
      className={`inline-flex items-center font-medium rounded ${sizeClasses[size]}`}
      style={{ color: color.text, backgroundColor: color.bg }}
      title={title}
    >
      {rating}
      {showTitle && <span className="ml-1 text-xs opacity-75">{title}</span>}
    </span>
  );
}