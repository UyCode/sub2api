# Mintlify 图片模型文档子项目

本仓库的接口文档站位于 `docs-mintlify/`，是独立 Mintlify 子项目，不依赖 `backend/` 或 `frontend/` 业务代码。

## 本地开发

```bash
cd docs-mintlify
npm install
npm run dev
```

也可以直接使用 Mintlify CLI：

```bash
cd docs-mintlify
npm run dev
```

本地预览默认会启动在 `http://localhost:3000`。如果端口被占用，Mintlify 会提示实际端口。

## 校验

```bash
cd docs-mintlify
npm run check
```

`npm run check` 会执行：

- `mint validate`：检查 `docs.json`、MDX 语法和构建配置。
- `mint broken-links`：检查站内断链。

提交前建议在仓库根目录额外运行：

```bash
git diff --check -- docs-mintlify DOCS-GUIDE.md
git diff -- docs-mintlify DOCS-GUIDE.md
```

## 文档范围

当前只覆盖 3 个图片模型：

| 产品展示名 | API 模型 ID | 接口族 |
| --- | --- | --- |
| GPT-Image-2 | `gpt-image-2` | OpenAI Images |
| Nano Banana 2 | `gemini-3.1-flash-image-preview` | Gemini Native |
| Nano Banana Pro | `gemini-3-pro-image-preview` | Gemini Native |

GPT-Image-2 使用：

- `POST /v1/images/generations`
- `POST /v1/images/edits`

Nano Banana 2 / Pro 使用：

- `POST /v1beta/models/{model}:generateContent`

Gemini Native 的编辑图不是独立 `/edits` 路由，而是在 `contents[].parts[]` 中加入图片 part，例如 `inlineData` 或上传服务返回的 `fileData`。

## 发布前替换项

- 将文档里的 `https://YOUR_DOMAIN` 替换为实际中转站域名。
- 确认 `Authorization: Bearer $SUB2API_API_KEY` 是否符合对外命名。
- 如价格变化，按 `DOCS-INTERNAL-MAINTENANCE.md` 更新内部价格配置，再运行 `node docs-internal/sync-pricing.mjs` 和 `npm run check`。
- 如果后端状态码或 failover 策略变化，同步更新 `reference/errors.mdx` 和 `reference/faq.mdx`。

## 内容来源口径

文档内容以本站中转站接口口径重新整理，参考 APIYI 的模型能力说明、BananaRouter 的状态码/模型表述，以及当前 sub2api 对 OpenAI Images、Gemini Native、图片尺寸档位和账号切换的实现事实。不要直接大段复制第三方原文。
