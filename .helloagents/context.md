# 项目上下文

Kiro-Go 是 Go 1.21+ HTTP 服务，将 Kiro 账号转换为 OpenAI / Anthropic 兼容 API。

关键入口：
- main.go：启动服务、加载配置、命令行参数
- config/config.go：全局配置、账号结构与持久化
- proxy/：OpenAI / Claude 转换、Kiro 上游请求与管理 API
- uth/：认证、Token 刷新、凭据导入
- web/：管理面板静态资源

当前新增能力：支持 clientMode，可在 kiro-ide 与 kiro-cli 间切换。

最后更新：2026-05-28 22:49:57
