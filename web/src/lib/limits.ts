// 各个输入的长度和数量上限。数值必须和后端一致（后端才是最终的校验方），
// 网页用它们限制输入框的长度、提前提示；limits.test.ts 会读取后端源码核对，后端改了数值这里会测试失败。

/** 看板名称的最大长度 */
export const BOARD_NAME_MAX = 100
/** 看板说明的最大长度 */
export const BOARD_DESC_MAX = 2000
/** 看板编号前缀的最小、最大长度 */
export const SLUG_MIN = 2
export const SLUG_MAX = 10

/** 列名称、说明、agent 指令的最大长度，以及一块看板最多几列 */
export const COLUMN_NAME_MAX = 60
export const COLUMN_INSTRUCTION_MAX = 4000
export const COLUMNS_PER_BOARD_MAX = 30

/** 工程师名称、角色提示词的最大长度，以及一块看板最多几位工程师 */
export const ENGINEER_NAME_MAX = 30
export const ENGINEER_PROMPT_MAX = 2000
export const ENGINEERS_PER_BOARD_MAX = 20

/** 卡片标题、描述的最大长度 */
export const CARD_TITLE_MAX = 200
export const CARD_DESC_MAX = 20000
/** 一张卡片最多几个标签，每个标签最多几个字符 */
export const CARD_TAGS_MAX = 10
export const CARD_TAG_MAX = 30
/** 一张卡片最多依赖几张卡片 */
export const CARD_DEPS_MAX = 20
/** 分支名、链接的最大长度 */
export const CARD_BRANCH_MAX = 200
export const CARD_URL_MAX = 500
/** 优先级的最大值（0 表示未设置） */
export const CARD_PRIORITY_MAX = 3

/** 评论的最大长度 */
export const COMMENT_MAX = 20000

/** 自动化规则：名称长度、动作个数、每块看板的规则个数、"添加评论"动作的文字长度、"设置截止"动作的最大天数 */
export const RULE_NAME_MAX = 100
export const RULE_ACTIONS_MAX = 20
export const RULES_PER_BOARD_MAX = 50
export const RULE_COMMENT_MAX = 2000
export const RULE_DUE_DAYS_MAX = 365
/** 规则条件（筛选语法）的最大长度 */
export const FILTER_MAX = 500

/** 搜索一次最多取回的卡片数 */
export const SEARCH_LIMIT = 500

/** 访问口令的最大长度（只允许可打印 ASCII，不含空格） */
export const PASSCODE_MAX = 128
