import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { zhCN } from '@/i18n/locales/zh-CN'
import { zhTW } from '@/i18n/locales/zh-TW'
import i18n from '@/i18n'
import { OfflineBanner } from './OfflineBanner'

vi.mock('./useOnlineStatus', () => ({
  useOnlineStatus: vi.fn(() => true),
}))

import { useOnlineStatus } from './useOnlineStatus'

describe('OfflineBanner', () => {
  it('在线时不显示', () => {
    vi.mocked(useOnlineStatus).mockReturnValue(true)
    const { container } = render(<OfflineBanner />)
    expect(container).toBeEmptyDOMElement()
  })

  it('离线时显示英文提示', async () => {
    vi.mocked(useOnlineStatus).mockReturnValue(false)
    await i18n.changeLanguage('en')
    render(<OfflineBanner />)
    expect(screen.getByRole('status')).toHaveTextContent(en.pwa.offlineBanner)
  })

  it('四种语言文案均非空', () => {
    expect(en.pwa.offlineBanner.length).toBeGreaterThan(0)
    expect(zhCN.pwa.offlineBanner.length).toBeGreaterThan(0)
    expect(zhTW.pwa.offlineBanner.length).toBeGreaterThan(0)
    expect(ja.pwa.offlineBanner.length).toBeGreaterThan(0)
  })
})