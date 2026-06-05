import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const docsRoot = path.join(repoRoot, "docs-mintlify");
const configPath = path.join(__dirname, "image-pricing.config.json");
const config = JSON.parse(fs.readFileSync(configPath, "utf8"));

const money = (value) => `$${Number(value).toFixed(3)}/${config.unitLabel}`;
const modelById = new Map(config.models.map((model) => [model.modelId, model]));

function replaceMarkedBlock(relativePath, marker, content) {
  const filePath = path.join(docsRoot, relativePath);
  const start = `{/* ${marker}:start */}`;
  const end = `{/* ${marker}:end */}`;
  const source = fs.readFileSync(filePath, "utf8");
  const pattern = new RegExp(`${escapeRegExp(start)}[\\s\\S]*?${escapeRegExp(end)}`);

  if (!pattern.test(source)) {
    throw new Error(`Missing pricing marker "${marker}" in ${relativePath}`);
  }

  const next = source.replace(pattern, `${start}\n${content.trim()}\n${end}`);
  if (next !== source) {
    fs.writeFileSync(filePath, next);
    console.log(`updated ${relativePath}#${marker}`);
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function publicPriceTable() {
  const rows = config.models.map((model) => {
    const shortNote = model.displayName === "Nano Banana 2"
      ? "生成/编辑同价；512px 为 $0.025/次"
      : model.displayName === "Nano Banana Pro"
        ? "生成/编辑同价；当前三档同价"
        : "生成/编辑同价";
    return `| ${model.displayName} | ${money(model.prices["1K"])} | ${money(model.prices["2K"])} | ${money(model.prices["4K"])} | ${shortNote} |`;
  });

  return [
    "| 模型 | 1K | 2K | 4K | 计费口径 |",
    "| --- | ---: | ---: | ---: | --- |",
    ...rows,
  ].join("\n");
}

function overviewPriceTable(modelId) {
  const model = modelById.get(modelId);
  if (!model) throw new Error(`Unknown model ${modelId}`);

  const rows = Object.entries(model.prices).map(([tier, price]) => {
    return `| \`${tier}\` | ${money(price)} | ${tier === "4K" ? "高分辨率或终稿素材" : tier === "2K" ? "高清物料" : "常规生成与编辑"} |`;
  });

  if (model.extraTiers) {
    for (const [tier, price] of Object.entries(model.extraTiers)) {
      rows.unshift(`| \`${tier}\` | ${money(price)} | 低分辨率预览档 |`);
    }
  }

  return [
    "| 档位 | 当前价格 | 说明 |",
    "| --- | ---: | --- |",
    ...rows,
  ].join("\n");
}

replaceMarkedBlock("index.mdx", "pricing:public-table", publicPriceTable());
replaceMarkedBlock("reference/pricing.mdx", "pricing:public-table", publicPriceTable());
replaceMarkedBlock("models/gpt-image-2/overview.mdx", "pricing:gpt-image-2", overviewPriceTable("gpt-image-2"));
replaceMarkedBlock("models/nano-banana-2/overview.mdx", "pricing:nano-banana-2", overviewPriceTable("gemini-3.1-flash-image-preview"));
replaceMarkedBlock("models/nano-banana-pro/overview.mdx", "pricing:nano-banana-pro", overviewPriceTable("gemini-3-pro-image-preview"));
