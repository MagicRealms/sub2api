# 国产模型按人民币数值计价

核对日期：2026-11-16。本站余额充值倍率 `BALANCE_RECHARGE_MULTIPLIER=1`，充值人民币数值直接记入站内 `$` 余额。本次 DeepSeek、Kimi、GLM 价格按官方人民币数值配置：官方 ¥1 对应站内 $1 基价，不进行外汇换算，实际扣费仍乘现有分组/用户专属倍率。

## 配置与生效方式

- 源码内的可审查配置：[model-pricing-cny-nominal.json](../../deploy/model-pricing-cny-nominal.json)。
- 本机生效文件：`/root/sub2api-deploy/data/model_pricing_override.json`。
- 本机配置：`/root/sub2api-deploy/data/config.yaml` 的 `pricing.override_file: /app/data/model_pricing_override.json`。
- JSON 价格单位是「站内余额 / token」。例如每百万输入 1，填写 `input_cost_per_token: 0.000001`。
- 覆盖文件已有热重载机制：修改有效 JSON 后，在 `pricing.hash_check_interval_minutes` 对应的检查周期内加载，未配置时为 10 分钟。格式错误保留上次有效价卡；远端目录更新后仍会叠加此覆盖文件。
- 下次调价维护此文件即可，不必反复改源码常量或重建镜像。源码中的 JSON 是本次配置快照，运行时使用挂载的数据文件。

「余额充值倍率」只控制充值到账，不能把模型的美元价自动换成人民币数值；「订阅 CNY 换算汇率」只控制订阅商品收款，不控制模型单价。

## 单价核对

以下为每百万 token 的站内基价，输入指缓存未命中；DeepSeek 列空闲价。

| 模型 | 输入 | 输出 | 缓存命中 | 缓存写入 5m / 1h |
| --- | ---: | ---: | ---: | ---: |
| DeepSeek Flash / V4.1 Flash 1017 | 1 | 4 | 0.02 | 不单列 |
| DeepSeek V4 Pro | 4.5 | 13.5 | 0.15 | 不单列 |
| Kimi K3 | 20 | 100 | 2 | 20 / 40 |
| Kimi K3.1 Pro | 13 | 68 | 1.3 | 13 / 26 |
| Kimi K3.1 Pro Highspeed | 26 | 136 | 1.3 | 26 / 52 |
| Kimi K2.8 Code（运营者指定参考 K2.7 Code） | 6.5 | 27 | 1.3 | 不单列 |
| GLM-5.2 / GLM-5.3 | 8 | 28 | 2 | 不单列 |
| GLM-5.3-Flash | 0.8 | 2.8 | 0.23 | 不单列 |

DeepSeek 继续沿用原有计费窗口：北京时间周一至周五 09:00–12:00、14:00–18:00 基价乘 2，周末整天空闲价。Flash 高峰输入/输出/缓存命中为 2 / 8 / 0.04；Pro 为 9 / 27 / 0.30。现有实现没有中国法定节假日历，本次未扩展日期规则。

现有分组倍率保留：DeepSeek 0.2、Kimi 0.3、GLM 0.35；组合分组及用户专属倍率按请求实际配置计算。例如 DeepSeek Flash 空闲输入每百万实付 $0.20，高峰 $0.40。此前的 $0.03 是美元基价 $0.15 乘以 0.2，和本次要求的人民币数值计价不同。

## 来源与范围

- [DeepSeek 中文官方定价](https://api-docs.deepseek.com/zh-cn/quick_start/pricing/)。
- [Kimi 中文官方定价](https://platform.kimi.com/docs/pricing/chat)。价格表是动态组件；核对官方页面内嵌的 MDX `DocTable` 数据，K3.1 Pro 数值为 13 / 68 / 1.3，而非国际站美元价。官方页面最后修改时间为 2026-11-12。
- [智谱官方人民币定价](https://docs.bigmodel.cn/cn/guide/start/pricing)。

Kimi K2.8 Code 没有查到独立官方 token 价，用户明确指定使用 K2.7 Code 的 6.5 / 27 / 1.3，配置中用 `pricing_note` 标记为本站售价。不能将此项表述为 K2.8 Code 官方定价。

配置覆盖当前三个账号所提供的全部 12 个模型名称，并补充 DeepSeek 官方名称/版本别名，共 15 条。新上游型号仍需核对并补充条目；未列入此文件的其他型号不自动按汇率放大。

## 程序修复与验证

原有 DeepSeek 模型策略会在目录解析和统一计费时强制回写美元默认价，盖掉 `override_file` 的显式价格。本次将本地输入、输出、缓存读取三项的覆盖标记逐字段传递到计费，只为没有显式覆盖的字段补默认价；显式 0 生效，`null` 仍表示删除字段。默认价卡峰谷因子、分组/渠道自定义价优先级保留。

验证覆盖配置加载、九种在售模型的真实计费入口与模型广场官方价、Kimi 5m/1h 缓存分账、DeepSeek 空闲/高峰及 0.2 倍率、部分字段覆盖/显式 0/null、热重载与删除回退，以及原有 DeepSeek/价格覆盖/广场回归测试。历史账单不重算。
