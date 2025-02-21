#!/bin/bash

# 创建目录结构
mkdir -p config model service data util

# 移动配置文件
mv core/shared/config/config.go config/
mv core/shared/config/const.go config/

# 移动模型文件
mv core/models/candle.go model/
mv core/models/maX.go model/
mv core/models/ticker.go model/

# 移动服务文件
mv core/services/service_context.go service/

# 移动数据层文件
mv core/datapipes/coaster.go data/
mv core/datapipes/pixel.go data/
mv core/datapipes/plate.go data/
mv core/datapipes/rsi.go data/
mv core/datapipes/segmentItem.go data/
mv core/datapipes/series.go data/
mv core/datapipes/shearForceGrp.go data/
mv core/datapipes/sorted.go data/
mv core/datapipes/tray.go data/

# 移动工具类文件
mv core/shared/util.go util/
mv core/shared/logging/writeLog.go util/

echo "文件迁移完成！新目录结构："
tree -d .

