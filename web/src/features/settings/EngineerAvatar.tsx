// 工程师的小头像：Q 版真人、猫咪两种画风，颜色和五官由种子随机决定，纯 SVG，不用图片文件。

import { useId } from 'react'
import { avatarTraits, type AvatarTraits } from './avatar'

interface Props {
  /** 随机种子：工程师的头像种子，没有就用编号 */
  seed: string
  /** 边长，单位像素 */
  size?: number
}

/** 真人的肤色、猫的毛色 */
const SKINS = ['#fbd9bf', '#f1c27d', '#d9a066', '#a8734a', '#7a4f33']
const FURS = ['#f6e3c4', '#ee9f55', '#a3a8b3', '#54545f', '#fbf7f2']
/** 发色 */
const HAIRS = ['#2b2233', '#5a3825', '#a8602c', '#e5b84d', '#c4495f', '#4a6fa5']
/** 眼睛、嘴巴的深色 */
const INK = '#2b2233'

/** Q 版大眼睛：深色椭圆加一点高光，笑眼样式画成弯弯的弧线 */
function BigEyes({ smile }: { smile: boolean }) {
  if (smile) return <path d="M13.2 23.5q2.3-3.4 4.6 0M22.2 23.5q2.3-3.4 4.6 0" fill="none" stroke={INK} strokeWidth="1.7" strokeLinecap="round" />
  return (
    <>
      <ellipse cx="15.5" cy="23" rx="1.9" ry="2.5" fill={INK} />
      <ellipse cx="24.5" cy="23" rx="1.9" ry="2.5" fill={INK} />
      <circle cx="16.2" cy="22" r="0.75" fill="white" />
      <circle cx="25.2" cy="22" r="0.75" fill="white" />
    </>
  )
}

/** 嘴巴：微笑、张嘴、猫嘴（w 形） */
function SmallMouth({ kind }: { kind: number }) {
  if (kind === 0) return <path d="M18 28q2 1.9 4 0" fill="none" stroke="#b4535b" strokeWidth="1.5" strokeLinecap="round" />
  if (kind === 1) return <ellipse cx="20" cy="28.4" rx="1.4" ry="1.2" fill="#b4535b" />
  return <path d="M17.8 27.6q1.1 1.3 2.2 0q1.1 1.3 2.2 0" fill="none" stroke="#b4535b" strokeWidth="1.4" strokeLinecap="round" />
}

/** 头发（脸后面的部分）：长发的两侧、丸子头的发髻 */
function HairBack({ t, color }: { t: AvatarTraits; color: string }) {
  if (t.hair === 1) return <rect x="7" y="13" width="26" height="21" rx="10" fill={color} />
  if (t.hair === 2) return <circle cx="20" cy="8.5" r="4.6" fill={color} />
  return null
}

/** 头发（脸前面的部分）：刘海 */
function HairFront({ t, color }: { t: AvatarTraits; color: string }) {
  if (t.hair === 3) return <path d="M8.5 20L9 11l5 4 3-6.5 4 6 4-6 2.5 6.5 4-4 .5 9c-3-4.6-8-6-11.5-5.8S11.5 15.4 8.5 20z" fill={color} />
  return <path d="M8.5 20C8.5 8.5 31.5 8.5 31.5 20c-3.5-4.5-8.5-6-11.5-5.600S12 15.5 8.5 20z" fill={color} />
}

/** Q 版真人：大头、大眼睛、腮红，发型、发色、肤色、眼镜随机 */
function Person({ t }: { t: AvatarTraits }) {
  const hair = HAIRS[t.hairColor]
  return (
    <>
      <path d="M5 40q0-9 15-9t15 9z" fill={`hsl(${t.hue2} 60% 42%)`} />
      <HairBack t={t} color={hair} />
      <ellipse cx="20" cy="22.5" rx="11.5" ry="11" fill={SKINS[t.skin]} />
      <HairFront t={t} color={hair} />
      <BigEyes smile={t.eyes === 2} />
      {t.glasses && <g fill="none" stroke={INK} strokeWidth="1.1"><circle cx="15.5" cy="23" r="3.6" /><circle cx="24.5" cy="23" r="3.6" /><path d="M19.1 23h1.8" /></g>}
      <circle cx="11.5" cy="27" r="1.8" fill="#f58ca3" fillOpacity="0.6" />
      <circle cx="28.5" cy="27" r="1.8" fill="#f58ca3" fillOpacity="0.6" />
      <SmallMouth kind={t.mouth} />
    </>
  )
}

/** 猫咪：圆脸、三角耳朵、鼻子和胡须 */
function Cat({ t }: { t: AvatarTraits }) {
  const fur = FURS[t.skin]
  return (
    <>
      <path d="M8.5 18L9.5 6l9 6zM31.5 18L30.5 6l-9 6z" fill={fur} />
      <path d="M11.5 14l.6-4.5 3.5 2.500zM28.5 14l-.6-4.5-3.5 2.500z" fill="#f4a6b5" />
      <ellipse cx="20" cy="23" rx="12.5" ry="10.5" fill={fur} />
      <BigEyes smile={t.eyes === 2} />
      <path d="M18.7 26.200h2.600l-1.3 1.400z" fill="#f4849a" />
      <SmallMouth kind={2} />
      <path d="M6 24l5 1M6 27l5-1M34 24l-5 1M34 27l-5-1" stroke={INK} strokeOpacity="0.45" strokeWidth="0.8" strokeLinecap="round" />
      <circle cx="11" cy="27" r="1.7" fill="#f58ca3" fillOpacity="0.55" />
      <circle cx="29" cy="27" r="1.7" fill="#f58ca3" fillOpacity="0.55" />
    </>
  )
}

/** 工程师头像 */
export function EngineerAvatar({ seed, size = 40 }: Props) {
  const gradientId = `avatar-${useId().replace(/:/g, '')}`
  const t = avatarTraits(seed)
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" aria-hidden="true" className="shrink-0 rounded-xl">
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor={`hsl(${t.hue} 72% 70%)`} />
          <stop offset="1" stopColor={`hsl(${t.hue2} 70% 58%)`} />
        </linearGradient>
      </defs>
      <rect width="40" height="40" fill={`url(#${gradientId})`} />
      {t.style === 'person' && <Person t={t} />}
      {t.style === 'cat' && <Cat t={t} />}
    </svg>
  )
}
