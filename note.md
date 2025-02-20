这个项目是用来在量化交易中定义一些基础类型和对一些走势特征进行判定的逻辑,帮看一下哪里有问题,需要修改或优化


cd /home/ubuntu/data/go/phyer.click/core && 
mkdir -p models utils config notify analysis market log &&
mv candle.go coaster.go plate.go tray.go models/ &&
mv util.go utils/ &&
mv config.go const.go config/ &&
mv dingding.go notify/ &&
mv maX.go rsi.go segmentItem.go series.go shearGorceGrp.go analysis/ &&
mv ticker.go market/ &&
mv writeLog.go log/

