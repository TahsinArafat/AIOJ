// web/src/components/RatingBadge.tsx
import { getRatingColor, getRatingTitle } from '../lib/rating';
import { useTheme } from '../context/ThemeContext';

interface RatingBadgeProps {
  rating: number;
  showTitle?: boolean;
  size?: 'sm' | 'md' | 'lg';
}

export default function RatingBadge({ rating, showTitle = false, size = 'md' }: RatingBadgeProps) {
  const color = getRatingColor(rating);
  const title = getRatingTitle(rating);
  const { theme } = useTheme();

  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };

  const textColor = theme === 'dark' && color.darkText ? color.darkText : color.text;
  const bgColor = theme === 'dark' && color.darkBg ? color.darkBg : color.bg;

  return (
    <span
      className={`inline-flex items-center font-medium rounded ${sizeClasses[size]}`}
      style={{ color: textColor, backgroundColor: bgColor }}
      title={title}
    >
      {rating}
      {showTitle && <span className="ml-1 text-xs opacity-75">{title}</span>}
    </span>
  );
}
