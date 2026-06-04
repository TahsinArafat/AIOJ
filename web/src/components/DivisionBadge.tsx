import { getDivisionName, getDivisionInfo, type Division } from '../lib/divisions';
import { useTheme } from '../context/ThemeContext';

interface DivisionBadgeProps {
  division: Division;
  size?: 'sm' | 'md' | 'lg';
}

export default function DivisionBadge({ division, size = 'md' }: DivisionBadgeProps) {
  const info = getDivisionInfo(division);
  const name = getDivisionName(division);
  const { theme } = useTheme();

  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };

  if (division === 0) {
    return (
      <span className={`inline-flex items-center font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 ${sizeClasses[size]}`}>
        {name}
      </span>
    );
  }

  const textColor = theme === 'dark' && info.darkColor ? info.darkColor : info.color;
  const bgColor = theme === 'dark' && info.darkBg ? info.darkBg : `${info.color}20`;

  return (
    <span
      className={`inline-flex items-center font-medium rounded ${sizeClasses[size]}`}
      style={{ color: textColor, backgroundColor: bgColor }}
    >
      {name}
    </span>
  );
}
