// 工程师头像的随机特征：由种子（头像种子或工程师编号）算出来，同一个种子永远得到同一张头像，不需要图片文件。

/** 头像的画风：Q 版真人、猫咪 */
export type AvatarStyle = 'person' | 'cat'

/** 头像的外观特征 */
export interface AvatarTraits {
  style: AvatarStyle
  /** 背景渐变的起始色相（0–359） */
  hue: number
  /** 背景渐变的结束色相 */
  hue2: number
  /** 脸色（真人是肤色，猫是毛色）：0–4 */
  skin: number
  /** 发型：0 短发、1 长发、2 丸子头、3 刺猬头 */
  hair: 0 | 1 | 2 | 3
  /** 发色：0–5 */
  hairColor: number
  /** 眼睛样式：2 是笑眼，其余是大眼睛 */
  eyes: 0 | 1 | 2
  /** 嘴巴样式：0 微笑、1 张嘴、2 猫嘴（猫固定用 2） */
  mouth: 0 | 1 | 2
  /** 真人有没有戴眼镜 */
  glasses: boolean
}

/** 画风的出现比例（占 10 份）：真人 7 份、猫 3 份 */
const STYLES: AvatarStyle[] = ['person', 'person', 'person', 'person', 'person', 'person', 'person', 'cat', 'cat', 'cat']

/** 把字符串散列成 32 位无符号整数（FNV-1a），再多搅拌几次让相邻种子的结果差别更大 */
function hash(seed: string): number {
  let h = 2166136261
  for (let i = 0; i < seed.length; i++) {
    h ^= seed.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  h ^= h >>> 15
  h = Math.imul(h, 2246822507)
  h ^= h >>> 13
  h = Math.imul(h, 3266489909)
  h ^= h >>> 16
  return h >>> 0
}

/** 按种子算出头像特征；各特征取散列值的不同部分，互相独立（用两轮散列凑够位数） */
export function avatarTraits(seed: string): AvatarTraits {
  const a = hash(seed)
  const b = hash(seed + '#')
  const hue = a % 360
  return {
    style: STYLES[(a >>> 9) % STYLES.length],
    hue,
    hue2: (hue + 35 + ((a >>> 18) % 50)) % 360,
    skin: b % 5,
    hair: ((b >>> 4) % 4) as AvatarTraits['hair'],
    hairColor: (b >>> 8) % 6,
    eyes: ((b >>> 12) % 3) as AvatarTraits['eyes'],
    mouth: ((b >>> 16) % 3) as AvatarTraits['mouth'],
    glasses: ((b >>> 22) & 3) === 0,
  }
}

/** 生成一个新的随机头像种子（只含小写字母和数字，不超过 8 位，满足后端的格式限制） */
export function randomAvatarSeed(): string {
  return Math.random().toString(36).slice(2, 10) || 'x'
}
