# client-mode 模块

## 目标

从 hank9999/kiro.rs#128 移植 Kiro CLI 客户端模拟能力到 Go 项目。

## 实现要点

- config.ClientMode：支持 kiro-ide / kiro-cli。
- 全局配置字段：clientMode、kiroCliVersion。
- 账号级覆盖字段：Account.ClientMode。
- Kiro CLI 模式请求差异：
  - origin=KIRO_CLI
  - 当前用户消息与历史用户消息注入 envState
  - streaming/runtime User-Agent 使用 ws-sdk-rust/... app/AmazonQ-For-CLI
  - x-amzn-codewhisperer-optout=false
  - social token refresh 使用 User-Agent: Kiro-CLI、Accept: */*、Accept-Encoding: gzip
- CLI 导入：./kiro-go --import-kiro-cli [--kiro-cli-db path] 从 kiro-cli SQLite uth_kv 导入凭据。
- 管理面板：设置页可改全局模式，账号详情可单独覆盖。

## 验证

- go test -mod=mod ./...
- go build -mod=mod -o .tmp/kiro-go-test.exe .
- .tmp/kiro-go-test.exe --help

最后更新：2026-05-28 23:33:21


