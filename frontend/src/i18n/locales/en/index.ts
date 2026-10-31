import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import usageGuide from './usageGuide'
import testCenter from './testCenter'
import channelIQ from './channelIQ'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
  ...usageGuide,
  ...testCenter,
  ...channelIQ,
}
