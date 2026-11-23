# Slate MCP token baseline

Source snapshot `a8a7a03911cc7ab07dcfb481725c86063e5b9875` (`v0.1.41-65-ga8a7a03`), macOS arm64, Go 1.27.2, MCP protocol 2025-06-18, tiktoken 0.12.0 (`o200k_base`). The benchmark runs a fresh Slate process on loopback with temporary SQLite. The bilingual fixtures comprise an empty board and a four-card board, including one card with a 1,770-character description, five comments, and two dependencies.

## Reproduce

Build the baseline and progressive-disclosure retest in separate checkouts. The retest was recorded at `e5f8c54`. Use `98ca9e`, which contains the same source tree, and set the original build version in the binary.

```sh
# From a checkout of a8a7a03
go build -ldflags '-X main.buildVersion=v0.1.41-65-ga8a7a03' -o /tmp/slate-mcp-baseline .
# From a checkout of 98ca9e
go build -ldflags '-X main.buildVersion=e5f8c54' -o /tmp/slate-mcp-updated .
# From this checkout with Codex CLI 0.170.0
uv run --no-project --with 'tiktoken==0.12.0' python docs/measure_mcp_tokens.py --binary /tmp/slate-mcp-baseline --codex codex
uv run --no-project --with 'tiktoken==0.12.0' python docs/measure_mcp_tokens.py --binary /tmp/slate-mcp-updated --codex codex
```

`--codex` is optional. It directs the host to a local synthetic Responses server, which drives a real Slate `whoami` round trip. Timestamps and session metadata can shift wire counts slightly between runs. Request counts include the MCP JSON-RPC envelope. Reply counts include the envelope and any text plus structured duplication. Structured counts show compact `structuredContent` alone.

For the service's tool-discovery measurements alone, replace `--codex codex` with `--discovery-only`.

## Tool discovery

Each cell shows name / description / input schema / full individual definition in tokens, then the share of the view’s sum of full definitions. Standalone fields are tokenized separately.

| Tool | Work | Legacy management |
| --- | ---: | ---: |
| `claim_card` | 2 / 171 / 131 / 340 (6.17%) | — |
| `comment_card` | 2 / 49 / 94 / 181 (3.29%) | 2 / 49 / 94 / 181 (2.54%) |
| `create_board` | 2 / 97 / 230 / 365 (6.63%) | 2 / 97 / 230 / 365 (5.12%) |
| `create_card` | 2 / 69 / 288 / 395 (7.17%) | 2 / 34 / 525 / 597 (8.37%) |
| `create_column` | — | 2 / 77 / 161 / 276 (3.87%) |
| `create_rule` | — | 2 / 275 / 416 / 733 (10.27%) |
| `delete_card` | — | 2 / 19 / 51 / 108 (1.51%) |
| `delete_column` | — | 2 / 69 / 92 / 199 (2.79%) |
| `delete_comment` | — | 2 / 15 / 39 / 92 (1.29%) |
| `delete_rule` | — | 2 / 28 / 38 / 104 (1.46%) |
| `finish_card` | 2 / 183 / 352 / 573 (10.40%) | 2 / 183 / 352 / 573 (8.03%) |
| `get_board` | 2 / 196 / 37 / 265 (4.81%) | 2 / 196 / 37 / 265 (3.71%) |
| `get_card` | 2 / 79 / 51 / 162 (2.94%) | 2 / 79 / 51 / 162 (2.27%) |
| `get_image` | 2 / 47 / 56 / 135 (2.45%) | 2 / 47 / 56 / 135 (1.89%) |
| `list_boards` | 3 / 39 / 10 / 82 (1.49%) | 3 / 39 / 10 / 82 (1.15%) |
| `list_cards` | 2 / 358 / 100 / 490 (8.90%) | 2 / 358 / 100 / 490 (6.87%) |
| `list_events` | — | 2 / 68 / 88 / 188 (2.63%) |
| `move_card` | 2 / 99 / 139 / 276 (5.01%) | 2 / 99 / 139 / 276 (3.87%) |
| `next_card` | 2 / 281 / 153 / 472 (8.57%) | — |
| `pull_updates` | 2 / 178 / 92 / 308 (5.59%) | — |
| `release_card` | 2 / 85 / 51 / 174 (3.16%) | 2 / 85 / 51 / 174 (2.44%) |
| `reorder_columns` | — | 3 / 55 / 86 / 180 (2.52%) |
| `set_project_path` | 3 / 82 / 71 / 192 (3.49%) | — |
| `update_board` | — | 2 / 35 / 123 / 196 (2.75%) |
| `update_card` | 2 / 62 / 333 / 433 (7.86%) | 2 / 57 / 573 / 668 (9.36%) |
| `update_column` | — | 2 / 31 / 162 / 231 (3.24%) |
| `update_rule` | — | 2 / 36 / 414 / 488 (6.84%) |
| `upload_image` | 2 / 135 / 117 / 290 (5.27%) | 2 / 135 / 117 / 290 (4.06%) |
| `wait_for_updates` | 3 / 132 / 121 / 292 (5.30%) | — |
| `whoami` | 2 / 40 / 10 / 82 (1.49%) | 2 / 40 / 10 / 82 (1.15%) |

## Calls

Request arguments below omit the common JSON-RPC fields. The table uses the generated fixture IDs from one run. The script posts a comment to wake the waiting client.

| Call | Arguments | Request | Wire reply | Structured reply |
| --- | --- | ---: | ---: | ---: |
| `list_boards` | `{}` | 31 | 2251 | 1089 |
| `get_board` | `{"board_id":3}` | 33 | 7652 | 3747 |
| `list_cards` | `{"board_id":3,"query":"col:8 is:free -is:waiting -is:human"}` | 50 | 325 | 145 |
| `get_card_typical` | `{"card_id":3}` | 33 | 1481 | 720 |
| `next_card` | `{"board_id":3,"column_id":8}` | 38 | 1538 | 748 |
| `comment_card` | `{"card_id":3,"body":"研发自测通过。"}` | 41 | 175 | 73 |
| `finish_card` | `{"card_id":3,"branch":"slate/sample","summary":"已交付，请审查交接。"}` | 51 | 315 | 141 |
| `get_board_empty` | `{"board_id":2}` | 33 | 7636 | 3739 |
| `list_cards_empty` | `{"board_id":2}` | 33 | 55 | 13 |
| `list_cards_unfiltered` | `{"board_id":3}` | 33 | 1068 | 506 |
| `get_card_long` | `{"card_id":4}` | 33 | 3465 | 1688 |
| `next_card_empty` | `{"board_id":2,"column_id":3}` | 38 | 38 | 5 |
| `wait_first` | `{"board_id":3,"timeout_seconds":1}` | 39 | 93 | 32 |
| `wait_empty` | `{"board_id":3,"since":18,"timeout_seconds":1}` | 43 | 93 | 32 |
| `wait_event` | `{"board_id":3,"since":18,"timeout_seconds":3}` | 43 | 261 | 115 |

## Retest after progressive disclosure

The updated binary was built from `e5f8c54`. The retest uses the same protocol, tokenizer, host version, fixture, and tool-call sequence. The setup creates two fixture boards in addition to the service’s built-in default board. [Build the equivalent snapshot and repeat the measurement](#reproduce).

| Tool | Updated default | After explicit disclosure | Updated legacy management |
| --- | ---: | ---: | ---: |
| `claim_card` | 2 / 30 / 131 / 199 (4.87%) | 2 / 30 / 131 / 199 (3.07%) | — |
| `comment_card` | 2 / 13 / 94 / 145 (3.55%) | 2 / 13 / 94 / 145 (2.23%) | 2 / 13 / 94 / 145 (2.45%) |
| `create_board` | 2 / 97 / 230 / 365 (8.94%) | 2 / 97 / 230 / 365 (5.62%) | 2 / 97 / 230 / 365 (6.17%) |
| `create_card` | 2 / 25 / 288 / 351 (8.59%) | 2 / 25 / 288 / 351 (5.41%) | 2 / 34 / 525 / 597 (10.09%) |
| `create_column` | — | 2 / 77 / 161 / 276 (4.25%) | 2 / 77 / 161 / 276 (4.67%) |
| `create_rule` | — | 2 / 275 / 416 / 733 (11.29%) | 2 / 275 / 416 / 733 (12.39%) |
| `delete_column` | — | 2 / 69 / 92 / 199 (3.07%) | 2 / 69 / 92 / 199 (3.36%) |
| `delete_rule` | — | 2 / 28 / 38 / 104 (1.60%) | 2 / 28 / 38 / 104 (1.76%) |
| `enable_management` | 2 / 43 / 95 / 176 (4.31%) | 2 / 43 / 95 / 176 (2.71%) | — |
| `finish_card` | 2 / 35 / 352 / 425 (10.40%) | 2 / 35 / 352 / 425 (6.55%) | 2 / 35 / 352 / 425 (7.19%) |
| `get_board` | 2 / 28 / 59 / 119 (2.91%) | 2 / 28 / 59 / 119 (1.83%) | 2 / 28 / 59 / 119 (2.01%) |
| `get_card` | 2 / 21 / 51 / 104 (2.55%) | 2 / 21 / 51 / 104 (1.60%) | 2 / 21 / 51 / 104 (1.76%) |
| `get_image` | 2 / 47 / 56 / 135 (3.30%) | 2 / 47 / 56 / 135 (2.08%) | 2 / 47 / 56 / 135 (2.28%) |
| `list_boards` | 3 / 20 / 35 / 88 (2.15%) | 3 / 20 / 35 / 88 (1.36%) | 3 / 20 / 35 / 88 (1.49%) |
| `list_cards` | 2 / 65 / 100 / 197 (4.82%) | 2 / 65 / 100 / 197 (3.03%) | 2 / 65 / 100 / 197 (3.33%) |
| `move_card` | 2 / 23 / 139 / 200 (4.90%) | 2 / 23 / 139 / 200 (3.08%) | 2 / 23 / 139 / 200 (3.38%) |
| `next_card` | 2 / 39 / 153 / 230 (5.63%) | 2 / 39 / 153 / 230 (3.54%) | — |
| `pull_updates` | 2 / 40 / 92 / 170 (4.16%) | 2 / 40 / 92 / 170 (2.62%) | — |
| `release_card` | 2 / 26 / 51 / 115 (2.82%) | 2 / 26 / 51 / 115 (1.77%) | 2 / 26 / 51 / 115 (1.94%) |
| `reorder_columns` | — | 3 / 55 / 86 / 180 (2.77%) | 3 / 55 / 86 / 180 (3.04%) |
| `set_project_path` | 3 / 24 / 71 / 134 (3.28%) | 3 / 24 / 71 / 134 (2.06%) | — |
| `update_board` | — | 2 / 35 / 123 / 196 (3.02%) | 2 / 35 / 123 / 196 (3.31%) |
| `update_card` | 2 / 19 / 333 / 390 (9.55%) | 2 / 19 / 333 / 390 (6.01%) | 2 / 57 / 573 / 668 (11.29%) |
| `update_column` | — | 2 / 31 / 162 / 231 (3.56%) | 2 / 31 / 162 / 231 (3.91%) |
| `update_rule` | — | 2 / 36 / 414 / 488 (7.52%) | 2 / 36 / 414 / 488 (8.25%) |
| `upload_image` | 2 / 135 / 117 / 290 (7.10%) | 2 / 135 / 117 / 290 (4.47%) | 2 / 135 / 117 / 290 (4.90%) |
| `wait_for_updates` | 3 / 32 / 121 / 192 (4.70%) | 3 / 32 / 121 / 192 (2.96%) | — |
| `whoami` | 2 / 18 / 10 / 60 (1.47%) | 2 / 18 / 10 / 60 (0.92%) | 2 / 18 / 10 / 60 (1.01%) |

The same call sequence retains the full replies for existing callers. The two optional arguments reduce the board reads while keeping the seven calls.

| Call | Baseline request / reply / structured | Updated request / reply / structured |
| --- | ---: | ---: |
| `list_boards` | 31 / 2251 / 1089 | 31 / 2249 / 1088 |
| `get_board` | 33 / 7652 / 3747 | 33 / 7658 / 3750 |
| `list_cards` | 50 / 325 / 145 | 50 / 325 / 145 |
| `get_card_typical` | 33 / 1481 / 720 | 33 / 1481 / 720 |
| `next_card` | 38 / 1538 / 748 | 38 / 1538 / 748 |
| `comment_card` | 41 / 175 / 73 | 41 / 175 / 73 |
| `finish_card` | 51 / 315 / 141 | 51 / 315 / 141 |
| `get_board_empty` | 33 / 7636 / 3739 | 33 / 7628 / 3735 |
| `list_cards_empty` | 33 / 55 / 13 | 33 / 55 / 13 |
| `list_cards_unfiltered` | 33 / 1068 / 506 | 33 / 1068 / 506 |
| `get_card_long` | 33 / 3465 / 1688 | 33 / 3465 / 1688 |
| `next_card_empty` | 38 / 38 / 5 | 38 / 38 / 5 |
| `wait_first` | 39 / 93 / 32 | 39 / 93 / 32 |
| `wait_empty` | 43 / 93 / 32 | 43 / 93 / 32 |
| `wait_event` | 43 / 261 / 115 | 43 / 261 / 115 |
| `list_boards` with compact option | — | 33 / 122 / 46 |
| `get_board` with compact option | — | 39 / 4088 / 2000 |

The empty and comment-triggered waits used the same cursor semantics and both returned the same wire token counts as the baseline. The Python event listener can run the wait outside the model turn. Agent callers still decide when to acknowledge the cursor.

## Host verification

| Client | Observed behavior |
| --- | --- |
| Go MCP SDK 1.8.0 | Real Streamable HTTP with local SQLite. Same-session list changes and explicit re-list expose the eight tools. A second connection with the same name and a reconnect stay at the work set. |
| Codex CLI / app-server 0.170.0 | Synthetic Responses capture showed the host continuing to send the default set after activation, including the next turn in one persistent thread. A scripted tool call requested the catalog and then created a column through the compatibility arguments in that same session. A REST read confirmed the column. |
| Claude Code, Cursor, OpenCode, Pi, Oh My Pi | Existing connect/launcher contract exercised by tests where applicable. Live host list refresh and real model runs were not included. |

The benchmark recorded the actual Codex outgoing Slate namespace, plus the object carrying the tool result. It applied o200k_base to these fields locally. The separate signed-in ChatGPT host trial on the synthetic fixture returned no usage telemetry; the available host rejected the requested gpt-5.4 model or timed out. This benchmark does not report provider billing or production usage.

## Discovery retest for task search

The tool description and `query` schema now advertise the search built into `list_cards`. Both snapshots expose the same tools and use the same query and response formats. The discovery retest used MCP 2025-06-18 and `o200k_base` (tiktoken 0.12.0) in an isolated SQLite workspace on macOS arm64 with Go 1.27.2.

| Discovery | Tools | Before (`f0d572`) | With search hints (`093f1d`) |
| --- | ---: | ---: | ---: |
| Default `tools/list` wire reply | 20 | 4,094 | 4,092 |
| Legacy management `tools/list` wire reply | 22 | 5,922 | 5,920 |
| `tools/list` wire reply after explicit disclosure | 28 | 6,493 | 6,491 |
| Full `list_cards` definition | | 197 | 195 |

Build the baseline from `f0d572`. Build the updated binary and run both measurements from the published checkout that contains the search change and this section.

```sh
# From the f0d572 checkout
go build -ldflags '-X main.buildVersion=slate-127-study' -o /tmp/slate-search-before .
# From the published checkout containing this section and the search change
go build -ldflags '-X main.buildVersion=slate-127-study' -o /tmp/slate-search-after .
uv run --no-project --with 'tiktoken==0.12.0' python docs/measure_mcp_tokens.py --discovery-only --binary /tmp/slate-search-before
uv run --no-project --with 'tiktoken==0.12.0' python docs/measure_mcp_tokens.py --discovery-only --binary /tmp/slate-search-after
```
