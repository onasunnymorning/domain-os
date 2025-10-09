import { formatDistanceToNow, format, isPast, isFuture, isWithinInterval } from 'date-fns';

export const formatPhaseDate = (dateString: string): string => {
  const date = new Date(dateString);
  return format(date, 'MMM d, yyyy');
};

export const formatPhaseDateLong = (dateString: string): string => {
  const date = new Date(dateString);
  return format(date, 'MMMM d, yyyy');
};

export const formatPhaseDateTime = (dateString: string): string => {
  const date = new Date(dateString);
  return format(date, 'MMM d, yyyy HH:mm');
};

export const formatRelativeDate = (dateString: string): string => {
  const date = new Date(dateString);
  const now = new Date();

  if (isPast(date)) {
    return `${formatDistanceToNow(date)} ago`;
  } else if (isFuture(date)) {
    return `in ${formatDistanceToNow(date)}`;
  }
  return 'now';
};

export const formatPhaseDuration = (start: string, end?: string | null): string => {
  if (!end) {
    return `Started ${formatPhaseDate(start)} (ongoing)`;
  }
  return `${formatPhaseDate(start)} - ${formatPhaseDate(end)}`;
};

export const isPhaseCurrent = (start: string, end?: string | null): boolean => {
  const now = new Date();
  const startDate = new Date(start);
  const endDate = end ? new Date(end) : null;

  if (endDate) {
    return isWithinInterval(now, { start: startDate, end: endDate });
  }
  // If no end date and start is in the past, it's ongoing (current)
  return isPast(startDate) || !isFuture(startDate);
};

export const isPhasePast = (start: string, end?: string | null): boolean => {
  if (!end) return false;
  return isPast(new Date(end));
};

export const isPhaseFuture = (start: string): boolean => {
  return isFuture(new Date(start));
};
