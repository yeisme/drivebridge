import React from "react";

export function BrandMark({
  size = 32,
  title = "DriveBridge",
}: {
  size?: number;
  title?: string;
}) {
  const tiny = size <= 24;
  return (
    <svg
      className="brand-mark"
      width={size}
      height={size}
      viewBox="0 0 128 128"
      role="img"
      aria-label={title}
    >
      <rect width="128" height="128" rx={tiny ? 28 : 28} fill="#214f43" />
      {tiny ? (
        <>
          <path
            d="M28 80c0-18 14-32 32-32h8c18 0 32 14 32 32"
            fill="none"
            stroke="#f1f5ee"
            strokeWidth="12"
            strokeLinecap="round"
          />
          <path
            d="M24 92h80"
            stroke="#d9efb7"
            strokeWidth="12"
            strokeLinecap="round"
          />
          <path d="M54 20h20l-5 16H59z" fill="#d9efb7" />
        </>
      ) : (
        <>
          <path
            d="M22 78c0-16 13-29 29-29h26c16 0 29 13 29 29"
            fill="none"
            stroke="#f1f5ee"
            strokeWidth="9"
            strokeLinecap="round"
          />
          <path
            d="M28 88h72"
            stroke="#d9efb7"
            strokeWidth="9"
            strokeLinecap="round"
          />
          <path
            d="M40 78V58M64 78V50M88 78V58"
            stroke="#f1f5ee"
            strokeWidth="8"
            strokeLinecap="round"
          />
          <path d="M56 24h16l-4 12h-8z" fill="#d9efb7" />
        </>
      )}
    </svg>
  );
}
