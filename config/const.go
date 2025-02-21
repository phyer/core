package core

const MAIN_ALLCOINS_PERIOD_MINUTES = 1
const MAIN_ALLCOINS_ONCE_COUNTS = 3
const MAIN_ALLCOINS_BAR_PERIOD = "3m"
const ALLCANDLES_PUBLISH = "allCandles|publish"
const ALLMAXES_PUBLISH = "allMaxes|publish"
const ALLCANDLES_INNER_PUBLISH = "allCandlesInner|publish"
const ORDER_PUBLISH = "private|order|publish"
const TICKERINFO_PUBLISH = "tickerInfo|publish"
const CCYPOSISTIONS_PUBLISH = "ccyPositions|publish"
const SUBACTIONS_PUBLISH = "subActions|publish"
const ORDER_RESP_PUBLISH = "private|actionResp|publish"

const CCYPOSITION_PUBLISH = "ccyPositions|publish"

const SORTED_VOL = "tickers|vol|sortedSet"
const SORTED_VOLDIFF = "tickers|volDiff|sortedSet"
const SORTED_VOLACCLER = "tickers|volAcclr|sortedSet"
const ALLCOASTERINFO_PUBLISH = "allCoasterInfo|publish"
const ALLSERIESINFO_PUBLISH = "allSeriesInfo|publish"

const SORTED_PRICE = "tickers|price|sortedSet"
const SORTED_PRICEDIFF = "tickers|priceDiff|sortedSet"
const SORTED_PRICEACCLR = "tickers|priceAcclr|sortedSet"
const SORTED_PRICEACCLR_PUBLISH = "tickers|priceAcclr|publish"
const SORTED_VOLSOFTPAIR = "volSoftPair"

const SORTED_CCYPOSITIONS = "private|positions|sortedSet"

const REMOTE_REDIS_PRE_NAME = "SARDINE_REMOTE_REDIS_"

const VERTICALLIMIT_ELEVATION = float64(-0.003)
const VERTICALLIMIT_INTERVAL = 3

// 倒推多少个周期去评估最近一段时间的历史曲线是否上升还是下降
const HISTORY_INTERVAL = 30

const LOG_BASE_PATH = "~/shared/"
const SHEARFORCE_VERTICAL_RATE = 100 //扭矩剪切率, 用于垂直极限统计
const REPORT_LEVEL = 3               // 上报到钉钉机器人需要的最低推荐指数
