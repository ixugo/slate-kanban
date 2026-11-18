// 让 t('login.title') 这样的调用有类型检查：键写错编译就报错。
import 'i18next'
import type { Messages } from './locales/en'

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: Messages }
  }
}
