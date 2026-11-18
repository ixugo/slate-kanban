import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import * as limits from './limits'

// 读出后端 internal/core 下所有非测试的 Go 源码，合成一个大字符串，方便按常量名查数值。
const coreDir = path.resolve(import.meta.dirname, '../../../internal/core')
function readGo(dir: string): string {
  return readdirSync(dir, { withFileTypes: true })
    .map((entry) => {
      const full = path.join(dir, entry.name)
      if (entry.isDirectory()) return readGo(full)
      return entry.name.endsWith('.go') && !entry.name.endsWith('_test.go') ? readFileSync(full, 'utf8') : ''
    })
    .join('\n')
}
const goSource = readGo(coreDir)

// 取后端常量的数值：写法是 `name = 数字`，可能带 `24 * 60` 这样的乘法，这里只支持单个数字
function goConst(name: string): number {
  const match = goSource.match(new RegExp(`\\b${name}\\s*=\\s*(\\d+)\\b`))
  if (!match) throw new Error(`后端找不到常量 ${name}`)
  return Number(match[1])
}

// 网页常量对应的后端常量名
const MAPPING: [keyof typeof limits, string][] = [
  ['BOARD_NAME_MAX', 'maxBoardNameLen'],
  ['BOARD_DESC_MAX', 'maxBoardDescLen'],
  ['PROJECT_PATH_MAX', 'maxProjectPathLen'],
  ['SLUG_MIN', 'minSlugLen'],
  ['SLUG_MAX', 'maxSlugLen'],
  ['COLUMN_NAME_MAX', 'maxColumnNameLen'],
  ['COLUMN_DESC_MAX', 'maxColumnDescLen'],
  ['COLUMN_INSTRUCTION_MAX', 'maxColumnInstructionLen'],
  ['COLUMNS_PER_BOARD_MAX', 'maxColumnsPerBoard'],
  ['CARD_TITLE_MAX', 'maxCardTitleLen'],
  ['CARD_DESC_MAX', 'maxCardDescLen'],
  ['CARD_TAGS_MAX', 'maxCardTags'],
  ['CARD_TAG_MAX', 'maxCardTagLen'],
  ['CARD_DEPS_MAX', 'maxWaitsOn'],
  ['CARD_BRANCH_MAX', 'maxCardBranchLen'],
  ['CARD_URL_MAX', 'maxCardURLLen'],
  ['CARD_PRIORITY_MAX', 'maxCardPriority'],
  ['COMMENT_MAX', 'maxCommentLen'],
  ['RULE_NAME_MAX', 'maxRuleNameLen'],
  ['RULE_ACTIONS_MAX', 'maxRuleActions'],
  ['RULES_PER_BOARD_MAX', 'maxRulesPerBoard'],
  ['RULE_COMMENT_MAX', 'maxRuleCommentLen'],
  ['RULE_DUE_DAYS_MAX', 'maxRuleDueDays'],
  ['FILTER_MAX', 'maxFilterLen'],
  ['SEARCH_LIMIT', 'maxListLimit'],
]

describe('输入上限和后端一致', () => {
  it.each(MAPPING)('%s 等于后端的 %s', (webName, goName) => {
    expect(limits[webName]).toBe(goConst(goName))
  })
})
