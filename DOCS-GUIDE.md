现在要生成我们中转站的文档，底层要用Mintlify作为文档生成，在这个项目下面创建一个子项目作为接口文档，你要做：
1. 生成的文档要针对GPT-Image-2 (生图和编辑图），Nano Banana 2 和 Nano Banana Pro (生图和编辑图）
2. 文档的内容可以从APIYI里面抄过来，但是涉及到状态码时，可以参考BananaRouter
3. 要求文档要非常专业，包括接入，参数，QA，价格，注意事项等
4. 目前只针对这3个图片模型的1k, 2k, 4k价格也要有说明
5. DOCS文档必须要非常专业，UI要非常美观，符合现代化。

---
参考页面如下：
网站：
https://docs.apiyi.com
https://bananarouter.com/docs/models

GPT-Image-2:
概览：https://docs.apiyi.com/api-capabilities/gpt-image-2/overview
文生图：https://docs.apiyi.com/api-capabilities/gpt-image-2/text-to-image
编辑图：https://docs.apiyi.com/api-capabilities/gpt-image-2/image-edit

Nano-Banana-2:
概览：https://docs.apiyi.com/api-capabilities/nano-banana-2-image/overview
文生图：https://docs.apiyi.com/api-capabilities/nano-banana-2-image/text-to-image
编辑图：https://docs.apiyi.com/api-capabilities/nano-banana-2-image/image-edit

Nano-Banana-Pro:
概览：https://docs.apiyi.com/api-capabilities/nano-banana-image/overview
文生图：https://docs.apiyi.com/api-capabilities/nano-banana-image/text-to-image
编辑图：https://docs.apiyi.com/api-capabilities/nano-banana-image/image-edit

---

状态码，一些表格等信息可以参考https://bananarouter.com/docs/models页面
我上次尝试的Demo在https://gruogu.mintlify.app/，你可以参考，但要比demo更强，更专业，UI要更漂亮
