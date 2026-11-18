// 界面文案在 React 里的统一入口：组件、页面一律从这里取 useTranslation 和 TFunction，
// 不直接引用底层的多语言库，这样以后换更轻的实现时，调用方一行都不用改。
export { useTranslation } from 'react-i18next'
export type { TFunction } from 'i18next'
