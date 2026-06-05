# 图片模型文档内部维护说明

> 内部文档：不要发布到客户文档站，不要加入 `docs-mintlify/docs.json` 导航。

## 价格维护

客户可见价格表由内部配置生成，配置文件位于：

```text
docs-internal/image-pricing.config.json
```

当前模型价格：

| 模型 | 1K | 2K | 4K | 额外档位 |
| --- | ---: | ---: | ---: | --- |
| GPT-Image-2 | `$0.060/次` | `$0.090/次` | `$0.150/次` | - |
| Nano Banana 2 | `$0.035/次` | `$0.045/次` | `$0.070/次` | `512px = $0.025/次` |
| Nano Banana Pro | `$0.090/次` | `$0.090/次` | `$0.090/次` | - |

修改价格后运行：

```bash
node docs-internal/sync-pricing.mjs
cd docs-mintlify
npm run check
```

`docs-internal/sync-pricing.mjs` 会同步以下客户页面中的价格表：

- `index.mdx`
- `reference/pricing.mdx`
- `models/gpt-image-2/overview.mdx`
- `models/nano-banana-2/overview.mdx`
- `models/nano-banana-pro/overview.mdx`

同步区域使用 MDX 注释标记：

```mdx
{/* pricing:public-table:start */}
...
{/* pricing:public-table:end */}
```

不要把 `image-pricing.config.json`、同步脚本、构建命令、部署流程、渠道倍率、分组倍率、账号成本等内部信息写到客户可见页面里。

## 客户站内容边界

客户文档站只展示：

- 如何接入 DeepToken 图片服务
- Base URL、鉴权、请求端点和示例代码
- 模型 ID、产品名、参数、请求体和响应体
- 当前公开价格和计费口径
- 常见问题、错误码、重试和超时建议
- URL 形式图片返回、base64 兼容说明、未来图片上传服务说明

客户文档站不展示：

- Mintlify 打包、部署、配置维护方式
- 价格配置文件路径
- sub2api 内部成本字段、账号成本统计、分组倍率、渠道倍率
- 内部账号池、账号成本、利润核算和供应商账务口径

## 发布前检查

```bash
cd docs-mintlify
npm run check
```

发布前额外确认：

- 搜索客户页面中没有 `pricing.config`、`sync:pricing`、`npm run`、`部署`、`打包` 等内部维护字样。
- `docs-mintlify/docs.json` 不引用内部维护文档。
- `DOCS-INTERNAL-MAINTENANCE.md` 和 `docs-internal/` 只留在仓库内部，不复制到 Mintlify 发布目录。
