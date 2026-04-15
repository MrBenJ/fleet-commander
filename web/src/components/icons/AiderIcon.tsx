export function AiderIcon({ size = 14 }: { size?: number }) {
  // Pixel art recreation of the aider logo — a green blocky character on dark background.
  // Grid is 8x8, each filled cell is a <rect>.
  const dark = "#0a1a0a";
  const bright = "#00ff41";
  const mid = "#00cc33";
  const dim = "#009926";

  // Row-by-row pixel data: [col, row, color]
  const pixels: [number, number, string][] = [
    // Row 0 (top)
    [3, 0, dim],
    [4, 0, dim],
    // Row 1
    [2, 1, dim],
    [3, 1, mid],
    [4, 1, mid],
    [5, 1, dim],
    // Row 2
    [2, 2, mid],
    [3, 2, bright],
    [4, 2, bright],
    [5, 2, mid],
    // Row 3
    [3, 3, mid],
    [4, 3, mid],
    [5, 3, dim],
    // Row 4
    [4, 4, mid],
    [5, 4, bright],
    [6, 4, mid],
    // Row 5
    [3, 5, dim],
    [4, 5, bright],
    [5, 5, bright],
    [6, 5, mid],
    // Row 6
    [2, 6, dim],
    [3, 6, mid],
    [4, 6, bright],
    [5, 6, mid],
    // Row 7 (bottom)
    [2, 7, dim],
    [3, 7, dim],
    [4, 7, mid],
  ];

  const cellSize = 24 / 8; // 3 units per cell in a 24x24 viewBox

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <title>Aider</title>
      <rect width="24" height="24" rx="3" fill={dark} />
      {pixels.map(([col, row, color], i) => (
        <rect
          key={i}
          x={col * cellSize}
          y={row * cellSize}
          width={cellSize}
          height={cellSize}
          fill={color}
        />
      ))}
    </svg>
  );
}
