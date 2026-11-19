# Slate

**harness engineering과 loop engineering을 위한 칸반 보드.** 보드는 당신이 설계합니다: 열, 프롬프트, 동시 작업 수를 정하는 엔지니어. 코딩 에이전트는 그 위에서 루프를 돌립니다: 카드를 맡고, git worktree에서 작업하고, 검토로 넘기고, 다음 카드를 기다립니다.

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

## 설치

1. [최신 릴리스](https://github.com/ixugo/slate-kanban/releases/latest)를 열고 사용 중인 시스템에 맞는 패키지를 내려받습니다:

| 시스템 | 패키지 |
|---|---|
| macOS (Apple 실리콘) | `slate_<version>_darwin_arm64.tar.gz` |
| Linux (x86-64) | `slate_<version>_linux_amd64.tar.gz` |
| Linux (ARM64) | `slate_<version>_linux_arm64.tar.gz` |
| Windows (x86-64) | `slate_<version>_windows_amd64.zip` |

2. 압축을 풀고 실행합니다. Windows에서는 압축을 풀고 `slate.exe`를 실행합니다.

```sh
tar -xzf slate_*.tar.gz
./slate
```

3. [http://127.0.0.1:7070](http://127.0.0.1:7070)을 엽니다.

- macOS에서 파일을 열 수 없다고 하면 `xattr -d com.apple.quarantine slate`를 한 번 실행합니다.
- Slate를 백그라운드에서 실행하고 로그인할 때 자동으로 시작하려면(macOS와 Linux) `./slate -d`를 실행합니다. 중지하고 서비스를 삭제하려면 `./slate -stop`을 사용합니다.
- 새 릴리스가 나오면 브라우저에 알림이 뜹니다. **지금 업데이트**를 선택한 뒤 Slate를 다시 시작합니다.

## 시작하기

1. **새 프로젝트**를 선택하고 **개발 흐름** 템플릿을 그대로 사용합니다: **받은 편지함 → 할 일 → 진행 중 → 검토 → 완료**. 받은 편지함은 당신 몫이고, **할 일**과 **진행 중**은 에이전트가 관리합니다.
2. **에이전트** 페이지에서 한 줄 프롬프트를 복사해 Claude Code, Cursor, Codex에 전달합니다. MCP와 Slate skill이 설치됩니다.
3. 무엇을 만들고 어떻게 검증할지 적은 카드를 추가합니다. 준비가 끝나면 **할 일**로 옮깁니다.
4. 에이전트를 계속 실행해 둡니다: **에이전트** 페이지의 **상시 실행**에서 프로젝트를 고르고, 프로젝트 디렉터리에서 시작한 에이전트 세션에 프롬프트를 붙여 넣습니다(`screen`이나 `tmux` 안에서 실행하면 터미널을 닫아도 유지됩니다). 에이전트는 업데이트 대기, **할 일**에서 맡기, **진행 중**으로 옮기기, **검토**로 넘기기를 반복합니다.
5. **검토**에서 인수인계 내용을 확인합니다. 통과하면 **완료**로 옮기고, 수정이 필요하면 댓글을 남기고 **진행 중**으로 되돌립니다.

## harness 설계하기

- **열 프롬프트**(**설정 → 열**)는 각 열에서 에이전트가 해도 되는 일을 알려 줍니다.
- **엔지니어**(**설정 → 엔지니어**)는 동시 작업 슬롯입니다: 한 엔지니어는 한 번에 카드 한 장만 맡습니다. 풀스택, 프런트엔드, 백엔드, UI, 테스트 엔지니어를 채용할 수 있습니다.
- **자동화 규칙**(**설정 → 자동화**)은 이벤트가 생기면 카드를 옮기거나 필드를 설정합니다.

## 데이터와 접근

- 데이터는 `~/.slate-kanban/`에 저장됩니다. Slate를 중지한 뒤 `data.db`와 `resource/`를 함께 백업하세요.
- Slate는 `0.0.0.0:7070`에서 수신합니다. 접근 암호가 비어 있으면 접속할 수 있는 사람이 모든 보드를 읽고 편집할 수 있습니다. 공유하기 전에 `Server.HTTP.Passcode`를 설정하고, HTTPS 리버스 프록시 뒤에 두고, 호스트 이름을 `Server.HTTP.AllowedHosts`에 추가하세요. [설정 예시](configs/config.example.toml)를 참고하세요.

## 라이선스

[LICENSE](LICENSE)를 참고하세요.
