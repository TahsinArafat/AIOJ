// web/src/components/VirtualTimer.tsx
import { useState, useEffect, useRef } from 'react';

interface VirtualTimerProps {
  endsAt: string | Date;
  onComplete?: () => void;
  size?: 'sm' | 'md' | 'lg';
}

function getTimeRemaining(endsAt: Date): number {
  return Math.max(0, Math.floor((endsAt.getTime() - Date.now()) / 1000));
}

function formatTime(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
}

function getColorClass(seconds: number): string {
  if (seconds === 0) return 'text-red-700 font-bold';
  if (seconds < 300) return 'text-red-600';
  if (seconds < 600) return 'text-orange-600';
  if (seconds < 1800) return 'text-yellow-600';
  return 'text-green-600';
}

export default function VirtualTimer({ endsAt, onComplete, size = 'md' }: VirtualTimerProps) {
  const endsAtDate = typeof endsAt === 'string' ? new Date(endsAt) : endsAt;
  const [timeLeft, setTimeLeft] = useState(() => getTimeRemaining(endsAtDate));
  const completedRef = useRef(false);

  useEffect(() => {
    const interval = setInterval(() => {
      const remaining = getTimeRemaining(endsAtDate);
      setTimeLeft(remaining);

      if (remaining === 0 && !completedRef.current) {
        completedRef.current = true;
        onComplete?.();
        clearInterval(interval);
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [endsAtDate, onComplete]);

  const sizeClasses = {
    sm: 'text-lg',
    md: 'text-2xl',
    lg: 'text-4xl',
  };

  return (
    <span className={`inline-flex items-center font-mono ${sizeClasses[size]} ${getColorClass(timeLeft)} ${timeLeft > 0 && timeLeft < 60 ? 'animate-pulse' : ''}`}>
      <span className="mr-2">⏱</span>
      {formatTime(timeLeft)}
    </span>
  );
}
