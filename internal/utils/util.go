package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	logrus "github.com/sirupsen/logrus"
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"time"
)

func NextPeriod(curPeriod string, idx int) string {
	list := []string{
		"1m",
		"3m",
		"5m",
		"15m",
		"30m",
		"1H",
		"2H",
		"4H",
		"6H",
		"12H",
		"1D",
		"2D",
		"5D",
	}
	nextPer := ""
	for i := 0; i < len(list)-1; i++ {
		if list[i] == curPeriod {
			nextPer = list[i+idx]
		}
	}
	return nextPer
}
func OtherPeriod(curPeriod string, cha int) string {
	list := []string{
		"1m",
		"3m",
		"5m",
		"15m",
		"30m",
		"1H",
		"2H",
		"4H",
		"6H",
		"12H",
		"1D",
		"2D",
		"5D",
	}
	nextPer := ""
	for i := 0; i < len(list)-1; i++ {
		if list[i] == curPeriod {
			if i+cha > 0 && i+cha < len(list)-1 {
				nextPer = list[i+cha]
			}
		}
	}
	return nextPer
}
func ShaiziInt(n int) int {
	rand.Seed(time.Now().UnixNano())
	b := rand.Intn(n)
	return b
}
func Difference(slice1 []string, slice2 []string) []string {
	var diff []string

	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for _, s1 := range slice1 {
			found := false
			for _, s2 := range slice2 {
				if s1 == s2 {
					found = true
					break
				}
			}
			// String not found. We add it to return slice
			if !found {
				diff = append(diff, s1)
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}

	return diff
}

func JsonToMap(str string) (map[string]interface{}, error) {

	var tempMap map[string]interface{}

	err := json.Unmarshal([]byte(str), &tempMap)

	if err != nil {
		logrus.Error("Unmarshal err: ", err, str)
	}

	return tempMap, err
}
func Sqrt(x float64) float64 {
	z := 1.0
	for math.Abs(z*z-x) > 0.000001 {
		z -= (z*z - x) / (2 * z)
	}
	return z
}
func RecursiveBubble(ary []int64, length int) []int64 {
	if length == 0 {
		return ary
	}
	for idx, _ := range ary {
		if idx >= length-1 {
			break
		}
		temp := int64(0)
		if ary[idx] < ary[idx+1] { //改变成>,换成从小到大排序
			temp = ary[idx]
			ary[idx] = ary[idx+1]
			ary[idx+1] = temp
		}
	}
	length--
	RecursiveBubble(ary, length)
	return ary
}

func Decimal(value float64) string {
	va := fmt.Sprintf("%.5f", value)
	v1, _ := strconv.ParseFloat(va, 32)
	v1 = v1 * 100
	s1 := fmt.Sprintf("%.3f", v1)
	return s1 + "%"
}
func ShortNum(value float64) string {
	va := fmt.Sprintf("%.5f", value)
	v1, _ := strconv.ParseFloat(va, 32)
	v1 = v1 * 100
	s1 := fmt.Sprintf("%.3f", v1)
	return s1
}

func GetRandomString(l int) string {
	str := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func Shaizi(n int) bool {
	rand.Seed(time.Now().UnixNano())
	b := rand.Intn(n)
	if b == 1 {
		return true
	}
	return false
}

func Sishewuru(x float64, digit int) (number float64, err error) {
	baseDigit := 10
	if digit < 0 {
		return x, errors.New("错误的精度，不能小于0")
	} else {
		baseDigit = pow(baseDigit, digit+1)
	}
	betweenNumber := float64(5)
	return (math.Floor((x*float64(baseDigit) + betweenNumber) / 10)) / float64(baseDigit/10), err
}

func pow(x, n int) int {
	ret := 1 // 结果初始为0次方的值，整数0次方为1。如果是矩阵，则为单元矩阵。
	for n != 0 {
		if n%2 != 0 {
			ret = ret * x
		}
		n /= 2
		x = x * x
	}
	return ret
}

// 获取当前函数名字
func GetFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

func Md5V(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}
func ToString(val interface{}) string {
	valstr := ""
	if reflect.TypeOf(val).Name() == "string" {
		valstr = val.(string)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valstr = fmt.Sprintf("%f", val)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valstr = strconv.FormatInt(val.(int64), 16)
	} else if reflect.TypeOf(val).Name() == "int" {
		valstr = fmt.Sprintf("%d", val)
	}
	return valstr
}

func ToInt64(val interface{}) int64 {
	vali := int64(0)
	if reflect.TypeOf(val).Name() == "string" {
		vali, _ = strconv.ParseInt(val.(string), 10, 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		vali = int64(val.(float64))
	}
	return vali
}

func ToFloat64(val interface{}) float64 {
	valf := float64(0)
	if reflect.TypeOf(val).Name() == "string" {
		valf, _ = strconv.ParseFloat(val.(string), 64)
	} else if reflect.TypeOf(val).Name() == "float64" {
		valf = val.(float64)
	} else if reflect.TypeOf(val).Name() == "int64" {
		valf = float64(val.(int64))
	}
	return valf
}

func HashString(input string) string {
	// 计算SHA-256哈希值
	hash := sha256.Sum256([]byte(input))
	// 转换为十六进制字符串
	hashHex := hex.EncodeToString(hash[:])
	// 返回前20位
	return hashHex[:23]
}
