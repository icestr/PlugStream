# 贡献指南

感谢对 PlugStream 的关注，欢迎通过 Issue 和 Pull Request 参与改进。

## 反馈问题与建议

- 提交前请先在 [Issues](https://github.com/icestr/PlugStream/issues) 搜索是否已有相同问题。
- 报告 Bug 时请附带操作系统、Go 版本、FFmpeg 版本、复现步骤与相关日志。
- 若发现安全漏洞，请勿公开提交 Issue，请参阅 [SECURITY.md](SECURITY.md)。

## 提交代码流程

1. Fork 本仓库并克隆到本地：

```bash
git clone https://github.com/YOUR_USERNAME/PlugStream.git
cd PlugStream
```

2. 基于 `main` 创建功能或修复分支：

```bash
git checkout -b feat/your-feature
# 或
git checkout -b fix/your-fix
```

3. 完成开发后运行格式化、静态检查与测试：

```bash
make fmt
go vet ./...
go test -v ./...
```

4. 提交并推送分支，在 GitHub 上发起 Pull Request：

```bash
git commit -m "feat(api): 添加功能描述"
git push origin feat/your-feature
```

### 提交信息格式

推荐使用约定式提交前缀：

- `feat:` 新增功能
- `fix:` 修复问题
- `docs:` 文档修改
- `refactor:` 代码重构
- `test:` 测试调整
- `chore:` 构建、依赖或工具配置

## 代码规范

- 使用 `gofmt` 保持格式统一，文件统一使用 LF 换行符。
- 导出类型、函数与常量需附带清晰的注释。
- 不忽略返回错误，合理使用 `fmt.Errorf("...: %w", err)` 包装上下文信息。
- 新功能或修复建议同步更新对应文档（`README.md`、`README_ZH.md`、`docs/`）。

## 许可证

通过提交 Pull Request，即表示您同意您的贡献将以 [MIT License](LICENSE) 协议发布。
