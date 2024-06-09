package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	const pkg, notes = "package ", "\n/*\n\n\n\n\n*/"
	root, _ := os.Getwd()
	files := []string{"01.guide", "02.structure", "03.event_execution", "04.cache", "05.reliability", "06.cluster", "07.tips", "08.qa", "09.exam", "10.additional"}
	ids := [][2]int{{1, 1}, {2, 7}, {8, 14}, {15, 17}, {18, 25}, {26, 28}, {29, 32}, {1, 5}, {1, 3}, {1, 5}}
	const tag = false // Tag
	if !tag {
		return
	}
	for i, file := range files {
		os.Mkdir(file, os.ModePerm)
		str := strings.Split(file, ".")
		if len(str) < 2 { // 没数据
			break
		}
		id, _ := strconv.Atoi(str[0])
		pkgName := fmt.Sprintf("_%d_%s\n", id, str[1])
		for j := ids[i][0]; j <= ids[i][1]; j++ {
			fileName := fmt.Sprintf("%.2d", j)
			f, _ := os.Create(fmt.Sprintf("%s/%s/%s..go", root, file, fileName))
			os.WriteFile(f.Name(), []byte(pkg+pkgName+notes), 0)
		}
	}
}
