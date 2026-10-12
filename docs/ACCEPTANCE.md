# 构建与验收

## 单元与 view 测试（任意平台）

```sh
go test ./...
```

view 测试（ui.NewTester）无需窗口/显示器，CI 可直接跑。

## 真实集群验收（Linux 本机）

```sh
docker compose -f docker/compose.yml up -d
./docker/wait-ready.sh es8
./docker/wait-ready.sh es7

SEISMIC_ES8=http://elastic:seismic8test@127.0.0.1:9200 \
SEISMIC_ES7=http://127.0.0.1:9210 \
go test ./internal/es/ -run Acceptance -v
```

- es8：8.19.6，xpack security 开（basic auth，elastic/seismic8test），http://127.0.0.1:9200
- es7：7.17.29，security 关，http://127.0.0.1:9210
- 验收覆盖：ping/版本解析、索引创建/写入/搜索/设置/删除、CatIndices/CatNodes/CatShards/CatRecovery、SLM（7.17 支持）、_security/user（8.x）。

## macOS 包（在 Linux 上交叉构建）

```sh
go tool mygo build -platform darwin/universal
# → build/darwin-universal/Seismic.app 与 Seismic <version> universal.dmg
```

无 cgo，任意平台可交叉。分发给别人需要 Developer ID 签名+公证（在 Mac 上做）；自用可跳过。

## 同步到 MacBook 并验收

```sh
rsync -av --delete /home/kchou/Work/github/seismic/build/darwin-universal/ <你的mac>:~/seismic-build/
```

Mac 上：

```sh
xattr -cr ~/seismic-build/Seismic.app   # 解除隔离（未签名 app 首次运行）
open ~/seismic-build/Seismic.app
```

### M4 视觉验收清单

- 概览卡片渲染健康大字与分片/文档/存储数字
- 侧栏 vibrancy 半透明（macOS 材质），选中模块 teal 高亮
- 索引表格：排序/拖列宽/右键菜单（查看设置/刷新/打开/关闭/删除）
- 搜索：JSON 编辑器高亮/行号/括号匹配/Ctrl+Space 补全（含 mapping 字段徽标）
- REST 控制台：多 tab、历史记录入库（重启 app 后仍在）、保存查询
- Keychain：保存带密码的连接后，终端 `security find-generic-password -s dev.bernylinville.seismic -a <连接uuid> -w` 应能取回
- 暗色模式：系统切换深浅色，界面跟随
- 中英切换：工具栏语言菜单，全界面文案切换

## OpenSearch / serverless / AWS IAM

按 elasticvue 行为移植并有 httptest 用例，但未对真实 OpenSearch/serverless/AES 集群验证（ADR 0005）；SigV4 与 aws4fetch 逐字节对拍（internal/es/testdata/sigv4_vectors.json 由上游库生成）。
