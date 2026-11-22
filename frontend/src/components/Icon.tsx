// Набор SVG-иконок проекта. Стиль: контурные, 24×24, currentColor.
// Эмодзи в интерфейсе не используются.

import type { SVGProps } from 'react'

type P = SVGProps<SVGSVGElement>

function base(props: P): P {
  return {
    width: 24,
    height: 24,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 2,
    strokeLinecap: 'round',
    strokeLinejoin: 'round',
    'aria-hidden': true,
    ...props,
  }
}

export const IconPlay = (p: P) => (
  <svg {...base(p)}><polygon points="6 3 20 12 6 21 6 3" fill="currentColor" stroke="none" /></svg>
)

export const IconPause = (p: P) => (
  <svg {...base(p)}><rect x="6" y="4" width="4" height="16" rx="1" fill="currentColor" stroke="none" /><rect x="14" y="4" width="4" height="16" rx="1" fill="currentColor" stroke="none" /></svg>
)

export const IconUndo = (p: P) => (
  <svg {...base(p)}><path d="M9 14 4 9l5-5" /><path d="M4 9h10.5a5.5 5.5 0 0 1 5.5 5.5v0a5.5 5.5 0 0 1-5.5 5.5H11" /></svg>
)

export const IconPlus = (p: P) => (
  <svg {...base(p)}><path d="M12 5v14M5 12h14" /></svg>
)

export const IconTrash = (p: P) => (
  <svg {...base(p)}><path d="M3 6h18" /><path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" /><path d="M10 11v6M14 11v6" /></svg>
)

export const IconEdit = (p: P) => (
  <svg {...base(p)}><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" /></svg>
)

export const IconCheck = (p: P) => (
  <svg {...base(p)}><path d="M20 6 9 17l-5-5" /></svg>
)

export const IconX = (p: P) => (
  <svg {...base(p)}><path d="M18 6 6 18M6 6l12 12" /></svg>
)

export const IconClock = (p: P) => (
  <svg {...base(p)}><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3.5 2" /></svg>
)

export const IconUpload = (p: P) => (
  <svg {...base(p)}><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><path d="M17 8l-5-5-5 5" /><path d="M12 3v12" /></svg>
)

export const IconUsers = (p: P) => (
  <svg {...base(p)}><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></svg>
)

export const IconTrophy = (p: P) => (
  <svg {...base(p)}><path d="M6 9H4.5a2.5 2.5 0 0 1 0-5H6" /><path d="M18 9h1.5a2.5 2.5 0 0 0 0-5H18" /><path d="M4 22h16" /><path d="M10 14.66V17c0 .55-.47.98-.97 1.21C7.85 18.75 7 20.24 7 22" /><path d="M14 14.66V17c0 .55.47.98.97 1.21C16.15 18.75 17 20.24 17 22" /><path d="M18 2H6v7a6 6 0 0 0 12 0V2Z" /></svg>
)

export const IconWifi = (p: P) => (
  <svg {...base(p)}><path d="M5 13a10 10 0 0 1 14 0" /><path d="M8.5 16.5a5 5 0 0 1 7 0" /><path d="M2 8.82a15 15 0 0 1 20 0" /><circle cx="12" cy="20" r="1" fill="currentColor" /></svg>
)

export const IconWifiOff = (p: P) => (
  <svg {...base(p)}><path d="m2 2 20 20" /><path d="M8.5 16.5a5 5 0 0 1 7 0" /><path d="M2 8.82a15 15 0 0 1 4.17-2.65" /><path d="M10.66 5c4.01-.36 8.14.9 11.34 3.76" /><path d="M16.85 11.25a10 10 0 0 1 2.22 1.68" /><path d="M5 13a10 10 0 0 1 5.24-2.76" /><circle cx="12" cy="20" r="1" fill="currentColor" /></svg>
)

export const IconArrowRight = (p: P) => (
  <svg {...base(p)}><path d="M5 12h14" /><path d="m12 5 7 7-7 7" /></svg>
)

export const IconArrowLeft = (p: P) => (
  <svg {...base(p)}><path d="M19 12H5" /><path d="m12 19-7-7 7-7" /></svg>
)

export const IconFlag = (p: P) => (
  <svg {...base(p)}><path d="M4 15s1-1 4-1 5 2 8 1 4-1 4-1V3s-1 1-4 1-5-2-8-1-4 1-4 1z" /><line x1="4" x2="4" y1="22" y2="15" /></svg>
)

export const IconEye = (p: P) => (
  <svg {...base(p)}><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z" /><circle cx="12" cy="12" r="3" /></svg>
)

export const IconEyeOff = (p: P) => (
  <svg {...base(p)}><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" /><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c6.5 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" /><path d="M6.61 6.61A13.5 13.5 0 0 0 2 12s3.5 7 10 7a9.74 9.74 0 0 0 5.39-1.61" /><path d="m2 2 20 20" /></svg>
)

export const IconPackage = (p: P) => (
  <svg {...base(p)}><path d="M16.5 9.4 7.55 4.24" /><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /><path d="M3.29 7 12 12l8.71-5" /><path d="M12 22V12" /></svg>
)

export const IconCrown = (p: P) => (
  <svg {...base(p)}><path d="m2 4 3 12h14l3-12-6 7-4-7-4 7-6-7Z" /><path d="M5 20h14" /></svg>
)

export const IconCat = (p: P) => (
  <svg {...base(p)}><path d="M12 5c.67 0 1.35.09 2 .26 1.78-2 5.03-2.84 6.42-2.26 1.4.58-.42 7-.42 7 .57 1.07 1 2.24 1 3.44C21 17.9 16.97 21 12 21s-9-3-9-7.56c0-1.25.5-2.4 1-3.44 0 0-1.89-6.42-.5-7 1.39-.58 4.72.23 6.5 2.23A9.04 9.04 0 0 1 12 5Z" /><path d="M8 14v.5M16 14v.5" /><path d="M11.25 16.25h1.5L12 17l-.75-.75Z" fill="currentColor" /></svg>
)

export const IconGavel = (p: P) => (
  <svg {...base(p)}><path d="m14 13-7.5 7.5a1.5 1.5 0 0 1-2-2L12 11" /><path d="m16 16 6-6" /><path d="m8 8 6-6" /><path d="m9 7 8 8" /><path d="m21 11-8-8" /></svg>
)

export const IconShield = (p: P) => (
  <svg {...base(p)}><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C12.5 20.5 9 18 9 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C17.5 3.8 20 5 22 5a1 1 0 0 1 1 1z" transform="scale(0.96) translate(0.5 0.5)" /></svg>
)

export const IconQuestion = (p: P) => (
  <svg {...base(p)}><circle cx="12" cy="12" r="9" /><path d="M9.1 9a3 3 0 0 1 5.82 1c0 2-3 3-3 3" /><circle cx="12" cy="17" r="0.5" fill="currentColor" /></svg>
)

export const IconRefresh = (p: P) => (
  <svg {...base(p)}><path d="M3 12a9 9 0 0 1 15.5-6.4L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-15.5 6.4L3 16" /><path d="M3 21v-5h5" /></svg>
)

export const IconSkipForward = (p: P) => (
  <svg {...base(p)}><polygon points="5 4 15 12 5 20 5 4" fill="currentColor" stroke="none" /><line x1="19" x2="19" y1="5" y2="19" /></svg>
)

export const IconHistory = (p: P) => (
  <svg {...base(p)}><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" /><path d="M3 3v5h5" /><path d="M12 7v5l4 2" /></svg>
)

export const IconStop = (p: P) => (
  <svg {...base(p)}><rect x="5" y="5" width="14" height="14" rx="2" fill="currentColor" stroke="none" /></svg>
)

export const IconTv = (p: P) => (
  <svg {...base(p)}><rect x="2" y="7" width="20" height="15" rx="2" /><path d="m17 2-5 5-5-5" /></svg>
)

export const IconPhone = (p: P) => (
  <svg {...base(p)}><rect x="6" y="2" width="12" height="20" rx="2" /><path d="M12 18h.01" /></svg>
)

export const IconMinus = (p: P) => (
  <svg {...base(p)}><path d="M5 12h14" /></svg>
)

export const IconStar = (p: P) => (
  <svg {...base(p)}><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" /></svg>
)

export const IconVolume = (p: P) => (
  <svg {...base(p)}><path d="M11 5 6 9H2v6h4l5 4V5Z" /><path d="M15.54 8.46a5 5 0 0 1 0 7.07" /><path d="M19.07 4.93a10 10 0 0 1 0 14.14" /></svg>
)

export const IconSettings = (p: P) => (
  <svg {...base(p)}><circle cx="12" cy="12" r="3" /><path d="M12 1v3M12 20v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M1 12h3M20 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12" /></svg>
)
