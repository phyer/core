package core

type Pixel struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	YCandle   YCandle `json:"yCandle,omitempty"`
	Score     float64 `json:"Score"`
	TimeStamp int64   `json:"timeStamp"`
	// ListMap map
}
type YCandle struct {
	Open  float64 //	开盘价格
	High  float64 //	最高价格
	Low   float64 //	最低价格
	Close float64 //	收盘价格
}
type MyPixel struct {
	InstID string `json:"instID"`
	Period string `json:"period"`
	Pixel  *Pixel `json:"pixel"`
}
type PixelList struct {
	Count          int      `json:"count"`
	LastUpdateTime int64    `json:"lastUpdateTime"`
	UpdateNickName string   `json:"updateNickName"`
	Ctype          string   `json:"ctype"`
	List           []*Pixel `json:"pixel"`
}

func (pxl *PixelList) ReIndex() error {
	for k, v := range pxl.List {
		v.X = float64(k)
	}
	return nil
}

// 冒泡排序 按时间排序
func (pxl *PixelList) RecursiveBubbleS(length int, ctype string) error {
	if length == 0 {
		return nil
	}
	for idx, _ := range pxl.List {
		if idx >= length-1 {
			break
		}
		temp := Pixel{}

		pre := pxl.List[idx]
		nex := pxl.List[idx+1]
		daoxu := pre.TimeStamp < nex.TimeStamp
		if ctype == "asc" {
			daoxu = !daoxu
		}
		if daoxu { //改变成>,换成从小到大排序
			temp = *pxl.List[idx]
			pxl.List[idx] = pxl.List[idx+1]
			pxl.List[idx+1] = &temp
		}

	}
	length--
	pxl.RecursiveBubbleS(length, ctype)
	return nil
}

func (pxl *PixelList) RecursiveBubbleX(length int, ctype string) error {
	if length == 0 {
		return nil
	}
	for idx, _ := range pxl.List {
		if idx >= length-1 {
			break
		}
		temp := float64(0)

		pre := pxl.List[idx]
		nex := pxl.List[idx+1]
		daoxu := pre.X < nex.X
		if ctype == "asc" {
			daoxu = !daoxu
		}
		if daoxu { //改变成>,换成从小到大排序
			temp = pxl.List[idx].X
			pxl.List[idx].X = pxl.List[idx+1].X
			pxl.List[idx+1].X = temp
		}

	}
	length--
	pxl.RecursiveBubbleS(length, ctype)
	return nil
}
