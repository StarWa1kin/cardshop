# 商城主题实施与验证记录

更新：2026-09-29。代码已落地；当前站点正式配置未变，现有后端进程仍运行旧版本。本记录区分“实现完成”和“上线验收”。

## 1. 实现内容

- Admin Settings 原店面模板入口升级为商城主题，提供主题 1/2/3、亮暗缩略图、预览、当前/待保存状态和独立应用按钮。保存失败保留选择；读取失败禁用应用并提供重试。
- 唯一新增保存结构为 `storefront_theme_config: {theme_id}`，只接受 `theme-1/theme-2/theme-3`；公开配置增加 `storefront_theme`。旧 `storefront_template`、`template_mode` 原样保留，不做数据迁移或双写。
- 解析顺序：有效新主题优先，其次旧 vault → 主题 2，其他 → 主题 1。非法提交返回错误并保留原值；异常存量值回退。
- 主题 3 使用石墨黑/冷白/青色配色、CSS 玻璃卡片主视觉、横向商品信息、响应式网格和轻量交互。配置已有横幅时优先原横幅。
- 交易/账户/内容页继续使用现有页面与主题变量，未复制第三套交易脚本。明暗继续沿用原偏好；预览只通过 URL，不写正式偏好。
- 购买弹层仅添加 UI 焦点指令、可访问名称与 CSS 钩子，支持 Esc、Tab 约束及关闭恢复焦点。
- Docker 增加 shared 目录复制，满足两个前端共享主题类型的构建路径。

## 2. 原业务保护

下列业务区域 `git diff` 无变更：商城 API、stores、composables、订单/支付/购物车业务页；后端商品/订单/支付模块。原路由路径与原业务守卫保留，新增守卫只传播主题预览参数。

`frontend/user/tests/storefrontTheme.test.ts` 使用改动前脚本 SHA-256 校验 Home、Products、VaultBannerHero、VaultProductCard、VaultProductListItem；ProductQuickBuy 去除新增的 UI 指令 import 后也须匹配原脚本。

这是源码不变性验证，不替代真实订单的端到端行为测试。本轮未创建订单、支付、登录或改变用户购物车。

## 3. 自动化与构建

运行目录：`/Users/2u/Desktop/MyProjects/cardshop`。前端命令使用 `PATH=/Users/2u/.nvm/versions/node/v22.23.2/bin:$PATH`。

| 阶段 | 命令 | 字面结果摘要 | 退出码 |
|---|---|---|---|
| 基线商城 | `pnpm --dir frontend/user test` | `# tests 62` / `# pass 62` / `# fail 0` | 0 |
| 基线后台 | `pnpm --dir frontend/admin test` | `Tests 28 passed (28)` | 0 |
| 基线前端构建 | 两端 `pnpm ... build` | `built` | 0 / 0 |
| 修改后商城 | `pnpm --dir frontend/user test` | `# tests 68` / `# pass 68` / `# fail 0` | 0 |
| 修改后后台 | `pnpm --dir frontend/admin test` | `Tests 33 passed (33)` | 0 |
| 修改后商城构建 | `pnpm --dir frontend/user build` | `✓ built in 5.19s` | 0 |
| 修改后后台构建 | `pnpm --dir frontend/admin build` | 详见原始日志 | 0 |
| Fullstack 构建 | `pnpm --dir frontend/admin build:fullstack` | `✓ built in 5.31s` | 0 |
| Go 回归 | `go test ./internal/modules/settings/... ./internal/architecture/...` | 所有有测试的包 `ok` | 0 |
| Go 构建 | `go build -o /tmp/cardshop-theme-audit/cardshop ./cmd/server` | 无输出 | 0 |
| 源码检查 | `git diff --check` | 无输出 | 0 |

原始 stdout/stderr 和命令输入归档：`docs/theme-delivery/verification.log`。Node 20 的首次基线测试因不支持 strip-types 退出 9，切换已安装 Node 22 后通过；Go 首次基线受网络/缓存权限影响退出 1，获批使用本机缓存后最终测试通过。没有把这些环境失败计作功能回归。

构建仍提示 Browserslist 数据较旧，未在此任务中升级依赖。主题 3 独立 CSS 约 12.81 kB / gzip 3.58 kB，采用现有 Vue、图标和 CSS，无新增动画依赖。未执行 Docker 镜像完整构建。

## 4. 浏览器验收

- 桌面：三个主题 × light/dark × card/list，共 12 项，主题标记、明暗、主区域和无横向溢出通过。记录 `docs/assets/theme-smoke-matrix.json`。
- 手机：同样 12 项，实际 `innerWidth=375`，`clientWidth=scrollWidth=365`。记录 `docs/assets/theme-mobile-smoke-matrix.json`。
- 平板/宽屏主题 3：768px 时 758/758；1440px 时 1430/1430，无横向溢出。
- 上述矩阵检查首页外壳，不是 24 组完整下单测试；真实商品加载后另外检查主题 3 首页、商品卡片、售罄/长名称/价格、桌面和手机快速购买面板。
- 快捷购买 Esc 关闭后焦点恢复 `aria-label=快速购买`，手机面板保持原数量/库存/金额与按钮。
- 从首页进入购物车保留主题预览参数并呈现空购物车；进入游客查单显示原邮箱/订单密码/可选订单号字段；退出预览移除主题参数并恢复 `theme-1`。
- 亮暗切换已可见验证；预览参数保留业务 query/hash 由单测覆盖。正式偏好不回写通过实现检查，未读取浏览器持久化私有状态。

卡片模式验收使用本地只读代理 `127.0.0.1:5193`：只替换公开配置响应中的 `template_mode=card`，GET 复用本机原数据，POST/PUT/PATCH/DELETE 返回 405。真实站点仍为 list，数据库未变。临时代理不属于发布产物。

实际截图（不是概念图）：

- `docs/assets/theme-3-dark-actual.png`
- `docs/assets/theme-3-light-actual.png`
- `docs/assets/theme-3-products-actual.png`
- `docs/assets/theme-3-quick-buy-actual.png`
- `docs/assets/theme-3-mobile-actual.png`
- `docs/assets/theme-3-mobile-quick-buy-actual.png`

## 5. 尚待发布环境验收

- 当前后台浏览器要求登录，未操作实际账号或保存正式主题。应用流程、失败处理和独立提交由组件测试及 Go 测试覆盖。
- 当前后端进程未重启，因此实际 `/public/config` 尚无新主题字段；前端兼容旧后端的回退与 URL 预览已经运行。
- 主站/分销站真实缓存失效、多账户权限、登录态、支付/退款/交付全链路、全部页面逐项视觉验收尚未执行。
- 当前数据中的品牌 Logo 与个别商品图片资源返回加载失败，截图保留实际状态；本轮未改资源 URL 或商品字段。
- 未量化全站对比度/性能指标，也未伪报概念稿中不存在的实时业务状态。

## 6. 启用与回退

1. 按现有发布流程部署新后端，再部署两端前端；先保留原配置，使用主题 3 URL 预览。
2. 登录后台 → 设置 → 商城主题 → 主题 3 → 应用主题。
3. 刷新商城验证生效；若希望截图中的卡片排列，使用原“商品展示设置”选择卡片并单独保存。主题与 card/list 是两个独立维度。
4. 回退 UI 首选重新应用主题 1 或 2；切换不会清空购物车或修改订单。旧版本前端继续读取未修改的旧模板字段。

源码交付（37 个文件，不包含依赖、编译目录或数据库）：

- 修改后源码：`docs/theme-delivery/modified-sources.tar.gz`
- 原始源码备份：`docs/theme-delivery/baseline-sources.tar.gz`
- 完整差异：`docs/theme-delivery/changes.patch`
- 原始/修改后 SHA-256：`docs/theme-delivery/manifest.json`
- 可运行源码回退：`docs/theme-delivery/rollback.py`

回退默认只检查，显式 `--apply` 才恢复源码；所有当前文件都匹配本次交付哈希后才写入，后续人工修改会中止回退。它不改数据库、构建输出或运行中进程，回退源码后仍需重建/重新部署。

```sh
python3 /Users/2u/Desktop/MyProjects/cardshop/docs/theme-delivery/rollback.py --root /Users/2u/Desktop/MyProjects/cardshop
# 确认需要恢复源码后，使用同一命令增加 --apply。
```

隔离临时目录演练结果见 `docs/theme-delivery/rollback-verification.log`：回退后全部基线哈希一致、重新应用差异后全部交付哈希一致、模拟后续修改时回退退出 1 且不覆盖。实际工作区仍保留完成后的代码。
