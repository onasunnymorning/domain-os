/**
 * Sol de Mayo icon — the sun from the Argentine flag.
 * Used as the light-mode icon in the theme switcher.
 *
 * Based on the heraldic sun with a face, 16 straight and 16 wavy rays.
 * Simplified for icon use at small sizes while retaining the distinctive
 * alternating ray pattern.
 */
export function SolDeMayo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 100 100"
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      aria-label="Sol de Mayo"
    >
      {/* Wavy rays (behind straight rays) */}
      {Array.from({ length: 16 }).map((_, i) => {
        const angle = (i * 360) / 16;
        return (
          <path
            key={`wavy-${i}`}
            d="M50,50 Q52,30 50,12 Q48,30 50,50"
            fill="currentColor"
            opacity={0.55}
            transform={`rotate(${angle + 11.25} 50 50)`}
          />
        );
      })}

      {/* Straight rays */}
      {Array.from({ length: 16 }).map((_, i) => {
        const angle = (i * 360) / 16;
        return (
          <polygon
            key={`ray-${i}`}
            points="50,10 48.2,42 50,40 51.8,42"
            fill="currentColor"
            transform={`rotate(${angle} 50 50)`}
          />
        );
      })}

      {/* Sun disc */}
      <circle cx="50" cy="50" r="18" fill="currentColor" />

      {/* Face — simplified for small sizes */}
      <g opacity="0.25">
        {/* Eyes */}
        <ellipse cx="44" cy="47" rx="2.2" ry="2.8" fill="white" />
        <ellipse cx="56" cy="47" rx="2.2" ry="2.8" fill="white" />
        {/* Pupils */}
        <circle cx="44.5" cy="47.5" r="1" fill="white" opacity="0.8" />
        <circle cx="56.5" cy="47.5" r="1" fill="white" opacity="0.8" />
        {/* Nose */}
        <path d="M49,50 L50,52 L51,50" fill="none" stroke="white" strokeWidth="0.6" />
        {/* Mouth */}
        <path d="M45,55 Q50,59 55,55" fill="none" stroke="white" strokeWidth="0.8" />
      </g>
    </svg>
  );
}
