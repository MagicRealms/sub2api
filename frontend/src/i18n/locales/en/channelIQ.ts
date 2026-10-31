export default {
  channelIQ: {
    title: 'Intelligence Status',
    description: 'Run a lightweight prompt through an account bound to each channel and verify real model response ability.',
    loadError: 'Failed to load intelligence status',
    runError: 'Channel intelligence test failed',
    footnote: 'Checks follow each channel interval; every probe creates one upstream request.',
    status: {
      disabled: 'Channel disabled',
      unconfigured: 'Not configured',
      pending: 'Testing',
      healthy: 'Healthy',
      warning: 'Needs attention',
      error: 'Unhealthy',
    },
    summary: { total: 'Channels', healthy: 'Healthy', pending: 'Testing', errors: 'Errors' },
    card: {
      noGroup: 'No group bound',
      autoPlatform: 'Automatic platform',
      model: 'Test model',
      autoModel: 'Automatic',
      latency: 'Latency',
      response: 'Latest response',
      waiting: 'Waiting for the first result…',
      checkedAt: 'Checked {time}',
      notChecked: 'Not checked yet',
      runNow: 'Run now',
    },
    empty: {
      title: 'No channel status yet',
      description: 'An administrator can enable Channel IQ in channel management and configure a model and prompt.',
    },
  },
}
