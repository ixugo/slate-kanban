import { BookOpenIcon, InfoIcon, LayoutTemplateIcon, SlidersHorizontalIcon } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { LogoutButton } from '@/features/auth/LogoutButton'
import { LANGUAGES } from '@/i18n/languages'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import { useTheme, type ThemeChoice } from '@/lib/theme-context'
import { AboutSettings } from './AboutSettings'
import { DesktopConnectionSettings } from './DesktopConnectionSettings'
import { TemplatesSettings } from './TemplatesSettings'
import { SkillsSettings } from './SkillsSettings'

interface Props {
  tab: string
  onTabChange: (tab: string) => void
  onClose: () => void
  onCloseAutoFocus: (event: Event) => void
}

const THEME_CHOICES = ['light', 'dark', 'system'] as const satisfies readonly ThemeChoice[]

const controlClass = 'h-9 w-full max-w-56 rounded-lg border border-input bg-field px-2.5 text-ui outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25'

export default function AppSettingsDialog({ tab, onTabChange, onClose, onCloseAutoFocus }: Props) {
  const { t } = useTranslation()
  const { lang, setLang } = useLanguage()
  const { theme, setTheme } = useTheme()
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        aria-describedby={undefined}
        className="flex h-[min(34rem,88dvh)] w-[min(36rem,96vw)] max-w-none flex-col gap-0 overflow-hidden p-0 sm:max-w-none"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader className="shrink-0 border-b border-border px-4 py-4 pr-14 sm:px-6">
          <DialogTitle>{t('nav.settings')}</DialogTitle>
        </DialogHeader>
        <Tabs
          value={tab}
          onValueChange={onTabChange}
          orientation="vertical"
          className="flex min-h-0 flex-1 gap-0 overflow-hidden"
        >
          <div className="w-24 shrink-0 border-r border-border bg-secondary/40 p-2 sm:w-40 sm:p-3">
            <TabsList aria-label={t('nav.settings')} className="w-full gap-1 bg-transparent p-0">
              <TabsTrigger value="general" className="min-h-9 justify-start rounded-lg px-2 sm:px-3">
                <SlidersHorizontalIcon aria-hidden className="hidden sm:block" />
                {t('preferences.general')}
              </TabsTrigger>
              <TabsTrigger value="templates" className="min-h-9 justify-start rounded-lg px-2 sm:px-3">
                <LayoutTemplateIcon aria-hidden className="hidden sm:block" />
                {t('preferences.templates')}
              </TabsTrigger>
              <TabsTrigger value="skills" className="min-h-9 justify-start rounded-lg px-2 sm:px-3">
                <BookOpenIcon aria-hidden className="hidden sm:block" />
                {t('preferences.skills')}
              </TabsTrigger>
              <TabsTrigger value="about" className="min-h-9 justify-start rounded-lg px-2 sm:px-3">
                <InfoIcon aria-hidden className="hidden sm:block" />
                {t('nav.about')}
              </TabsTrigger>
            </TabsList>
          </div>
          <TabsContent value="general" className="min-h-0 min-w-0 flex-1 space-y-5 overflow-y-auto p-4 sm:p-6">
            <label className="grid gap-1.5 font-medium">
              {t('language.label')}
              <select
                className={controlClass}
                value={lang}
                onChange={(event) => {
                  const next = LANGUAGES.find((language) => language.code === event.target.value)
                  if (next) setLang(next.code)
                }}
              >
                {LANGUAGES.map((language) => (
                  <option key={language.code} value={language.code} lang={language.code}>
                    {language.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="grid gap-1.5 font-medium">
              {t('theme.label')}
              <select
                className={controlClass}
                value={theme}
                onChange={(event) => {
                  const next = THEME_CHOICES.find((choice) => choice === event.target.value)
                  if (next) setTheme(next)
                }}
              >
                {THEME_CHOICES.map((choice) => (
                  <option key={choice} value={choice}>{t(`theme.${choice}`)}</option>
                ))}
              </select>
            </label>
            <DesktopConnectionSettings />
            <LogoutButton />
          </TabsContent>
          <TabsContent value="templates" className="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-6">
            <TemplatesSettings />
          </TabsContent>
          <TabsContent value="skills" className="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-6">
            <SkillsSettings />
          </TabsContent>
          <TabsContent value="about" className="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-6">
            <AboutSettings />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
