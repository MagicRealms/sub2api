export default {
  channelIQ: {
    title: '智商状态',
    description: '管理员会用一个上游 Key 定时测试模型能力，这里只展示最近六次测试结果。',
    loadError: '智商状态加载失败',
    footnote: '结果由管理员配置的测试任务自动更新，用户端仅可查看。',
    status: {
      disabled: '渠道已停用',
      unconfigured: '未配置测试',
      pending: '测试中',
      healthy: '能力正常',
      warning: '需要关注',
      error: '能力异常',
    },
    summary: { total: '测试记录', healthy: '能力正常', pending: '测试中', errors: '异常', records: '最近记录' },
    card: {
      noGroup: '未绑定分组',
      autoPlatform: '自动平台',
      model: '测试模型',
      autoModel: '自动选择',
      autoReasoning: '默认思考',
      latency: '响应耗时',
      response: '最近响应',
      waiting: '等待第一次测试结果…',
      checkedAt: '检测于 {time}',
      notChecked: '尚未检测',
    },
    empty: {
      title: '还没有渠道状态',
      description: '管理员可以在渠道管理中开启“渠道智商”，配置测试模型和提示词。',
    },
  },
}
