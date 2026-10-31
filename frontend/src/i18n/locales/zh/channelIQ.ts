export default {
  channelIQ: {
    title: '智商状态',
    description: '按渠道实际绑定的账号发起轻量问答，查看模型是否能正常理解并返回内容。',
    loadError: '智商状态加载失败',
    runError: '渠道智商测试失败',
    footnote: '状态会按渠道配置的间隔自动刷新；每次探测都会产生一次上游请求。',
    status: {
      disabled: '渠道已停用',
      unconfigured: '未配置测试',
      pending: '测试中',
      healthy: '能力正常',
      warning: '需要关注',
      error: '能力异常',
    },
    summary: { total: '渠道总数', healthy: '能力正常', pending: '测试中', errors: '异常' },
    card: {
      noGroup: '未绑定分组',
      autoPlatform: '自动平台',
      model: '测试模型',
      autoModel: '自动选择',
      latency: '响应耗时',
      response: '最近响应',
      waiting: '等待第一次测试结果…',
      checkedAt: '检测于 {time}',
      notChecked: '尚未检测',
      runNow: '立即测试',
    },
    empty: {
      title: '还没有渠道状态',
      description: '管理员可以在渠道管理中开启“渠道智商”，配置测试模型和提示词。',
    },
  },
}
