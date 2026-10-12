export default {
  usageGuide: {
    nav: '使用教程',
    eyebrow: '新手指南 · 从这里开始',
    title: '第一次使用中转站？',
    titleAccent: '跟着做，就能用。',
    description: '欢迎来到{site}。先创建一把 API 密钥，再把它填进你喜欢的客户端。我们把需要的地址、配置和操作都整理在这里了。',
    heroNote: '日常聊天、辅助写代码、接入自己的应用，都可以从这里开始。',
    getKey: '创建 API 密钥',
    chooseClient: '我已经有密钥了',
    login: '登录控制台',
    copy: '复制',
    copyCode: '复制{label}',
    copyPrompt: '复制测试问题',
    contents: '教程目录',
    previewLabel: '接入流程示意',
    previewComment: '# 接入只需要三项信息',
    previewReply: '你好，很高兴为你提供帮助。',
    testPrompt: '你好，请用一句话介绍你自己。',
    sections: { prepare: '准备密钥', clients: '客户端接入', verify: '验证成功', faq: '常见问题' },
    video: {
      title: '更喜欢看视频？跟着操作一遍',
      description: 'CC Switch 配置与使用教程，介绍添加模型供应商、切换 API 配置等操作，包含 Codex 的接入方式。',
      watch: '去 B 站看视频',
      note: '使用本站时，请选择 Codex，并将视频中的 API 地址、密钥和模型替换为本页提供的地址、你在本站创建的密钥和分组支持的模型。CC Switch 安装包可从下方 Codex 教程中的官方项目链接下载。'
    },
    prepareTitle: '先准备好这三件事',
    prepareHint: '所有客户端都从这一步开始',
    preparation: {
      account: { title: '登录并确认额度', text: '登录本站，在仪表盘确认余额或可用订阅额度。余额不足时，可通过侧边栏的“余额充值”或兑换入口补充。', action: '进入控制台' },
      key: { title: '创建自己的 API 密钥', text: '打开“API 密钥”，点击“创建密钥”，取个好记的名称，并选择需要使用的分组。复制生成的完整密钥。', action: '前往 API 密钥' },
      config: { title: '认准地址与模型', text: 'API Key 是刚才复制的那一串字符。Base URL 填本站 API 地址，模型名称按密钥分组支持的 Model ID 填写。', action: '查看密钥接入配置' }
    },
    groupNote: '分组决定这把密钥可以调用哪些模型，以及对应的计费方式。请按你的使用需求选择；如果分组注明“无 Astra”，就不能用该密钥调用 Astra 模型。',
    clientsTitle: '你想用它做什么？',
    clientsHint: '点击你使用的客户端，跟着下面的步骤配置。第一次体验聊天，可以从 Cherry Studio 开始。',
    clientSummaries: { cherry: '聊天、问答、日常 AI 助手', codex: '让 AI 帮你阅读和编写代码', opencode: '在终端里使用 AI 编程助手', api: '接入应用或检查 API 连通性' },
    noTerminal: '图形界面 · 按字段填写即可',
    os: '选择操作系统',
    downloadCherry: '下载 Cherry Studio',
    fields: { name: '服务商名称', type: '服务商类型', keyValue: '粘贴你在本站创建的完整密钥' },
    cherry: {
      addTitle: '安装客户端，添加一个服务商',
      addText: '下载并安装 Cherry Studio。打开“设置 → 模型服务”，添加服务商，类型选择 OpenAI，名称可以填写本站名称。不同版本的按钮名称可能略有区别。',
      fieldsTitle: '照着填写这些信息',
      urlNote: '这里填写根地址，不加 /v1 或 /responses，Cherry Studio 会补齐请求路径。若使用其他客户端，请按该客户端的要求填写地址。',
      chatTitle: '添加模型，开始第一段对话',
      chatText: '保存密钥后，同步/获取模型并添加需要的模型；也可以手动填写上方示例 Model ID。打开服务商开关，回到聊天页，选择刚刚添加的模型，发送一句“你好”。如果版本提供 OpenAI Responses 协议选项，可选用该协议。'
    },
    codex: {
      installTitle: '安装 Codex',
      installText: '如果还没安装 Node.js，先安装 LTS 版本，再打开终端执行下面的命令。已安装 Codex 的用户可直接继续。',
      easyTitle: '想少写配置？可以用 CC Switch',
      easyText: '安装 CC Switch 后，回到本站“API 密钥”，找到你的密钥，点击“导入到 CC Switch”，按提示导入并启用 Codex 供应商。也可以点击“使用密钥”查看本站生成的配置。',
      downloadSwitch: '下载 CC Switch',
      manualTitle: '手动配置方式（展开查看）',
      configText: '在你电脑的用户目录中找到下方文件；没有就新建 .codex 文件夹和 config.toml 文件。已有配置请先备份，再合并下面内容，避免重复同名配置项。Windows 可在资源管理器地址栏输入 %USERPROFILE%\\.codex 打开目录。',
      envText: '把下面的 YOUR_API_KEY 换成你的完整密钥，在同一个终端执行，然后启动 Codex。无需填写本站登录密码。',
      startTitle: '启用配置，开始对话',
      startText: '导入或修改配置后，完全退出再打开 Codex。CLI 用户在项目文件夹打开终端并运行 codex；先发送一个简单问题确认能收到回答，再开始编程。已有桌面版或编辑器扩展的用户同样需要重启客户端。'
    },
    envNote: '上面的环境变量只对当前终端及其启动的程序生效。关闭终端后需重新设置。需要长期使用时，可将变量保存到系统用户环境变量或自己的 shell 配置，再重新打开客户端；不要把密钥提交到项目仓库。',
    opencode: {
      installTitle: '安装 OpenCode',
      installText: '先安装 Node.js LTS，再在终端执行以下命令。已安装则跳过。',
      configTitle: '在项目中添加配置文件',
      configText: '在项目根目录新建 opencode.json，复制以下配置。已有文件请合并 provider 和 model 配置。本例使用 Responses 协议，密钥通过环境变量读取。',
      startTitle: '设置密钥并启动',
      startText: '把 YOUR_API_KEY 换成你的密钥，在项目目录的同一个终端执行。进入 OpenCode 后用 /models 检查并选择 Sub2API 下的模型，然后发送一个简单问题。'
    },
    api: {
      modelsTitle: '先检查密钥和模型列表',
      modelsText: '把 YOUR_API_KEY 换成你的完整密钥，复制命令到终端执行。能收到模型列表表示地址和鉴权通过；具体模型能否使用，还需要实际请求验证。',
      requestTitle: '发起一次最小对话',
      requestText: '在刚才的同一个终端执行。示例使用 /v1/responses 流式接口；看到文本增量和 response.completed 事件表示本次响应已完成。若模型不在你的分组中，请替换 model 字段。',
      billingNote: '模型列表查询不会生成回答；对话测试会产生正常用量和费用。Windows 示例使用 PowerShell 和系统自带的 curl.exe。'
    },
    modelNote: '本页用 gpt-6.1-sol 演示配置，不代表所有分组都开放此模型。以密钥“使用密钥”中的模型目录、客户端同步结果和本站分组说明为准；模型名称需要完整匹配。所有 YOUR_API_KEY 都要替换为你自己的密钥。',
    verifyTitle: '有回复，再回本站看一眼',
    verifyText: '客户端能正常收到回答后，打开本站“使用记录”，确认这次请求对应的模型、Token 和费用。记录可能稍有延迟，请刷新查看。',
    viewUsage: '查看使用记录',
    tryPrompt: '第一次对话，可以复制这个问题',
    verifyNote: '本页演示区是示意内容。是否接入成功，以你的客户端实际回答和本站用量记录为准。',
    faqTitle: '遇到问题，先看这里',
    faq: {
      key: { q: 'API Key 是什么？填登录密码可以吗？', a: 'API Key 是在本站“API 密钥”页面创建的调用凭证。请粘贴完整密钥，前后不要带空格。登录密码、其他平台的密钥都不能代替它。不要把完整密钥发给他人或放进公开截图。' },
      model: { q: '提示 model_not_allowed / model not found？', a: '先核对模型 ID 是否拼写正确，再检查这把密钥绑定的分组和模型限制。客户端展示某个模型不代表你的分组一定有权限；换成已开放的模型，或创建绑定合适分组的新密钥。' },
      auth: { q: '401 / 403，提示密钥或权限有问题？', a: '确认 API 地址指向本站，密钥来自本站且没有多余空格。再查看密钥是否停用、过期、额度用尽或被 IP 白名单限制。403 也可能来自分组或模型权限，请按完整报错排查。' },
      notFound: { q: '404 / 路径不存在，地址到底怎么填？', a: 'Codex 和 OpenCode 示例的 Base URL 以 /v1 结尾；Cherry Studio 按本页填写根地址。不要重复拼接 /v1/v1，也不要把完整的 /v1/responses 或 /v1/chat/completions 当作普通 Base URL。不同客户端填法有区别，请使用对应教程。' },
      rate: { q: '429、繁忙或没有可用账号，怎么办？', a: '先降低同时进行的对话或任务数量，稍后重试。检查自己的并发、速率和套餐额度。若仍持续报错，请把时间、客户端、模型和已隐藏密钥的报错信息发给管理员，以便检查渠道状态。' },
      balance: { q: '还有余额，为什么仍提示额度不足？', a: '除了账户余额，还要检查密钥自身的额度、过期时间，以及所选分组对应的订阅是否有效或达到周期上限。额度以账户、密钥和分组的实际限制共同决定。' },
      context: { q: '对话太长，提示 context_length_exceeded？', a: '新建对话，减少附带的历史内容或文件；编程客户端可先压缩上下文。调整客户端的上下文设置不会增加模型实际支持的窗口，超长对话也会增加费用。' },
      website: { q: '能直接在 ChatGPT 官网里填这个密钥吗？', a: '请使用支持自定义 API 地址的客户端或应用，例如本页的 Cherry Studio、Codex、OpenCode。中转站账户余额用于本站 API 调用，不等同于 ChatGPT 官网会员。' }
    },
    supportNote: '需要帮助时，请提供：发生时间、客户端名称、模型 ID、报错截图或文字。请隐藏完整 API 密钥；保留 request_id 有助于管理员定位问题。',
    sourcesNote: '按本站接入方式整理 · 客户端细节可查阅官方文档'
  }
}
