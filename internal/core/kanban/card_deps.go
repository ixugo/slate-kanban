package kanban

// maxWaitsOn 一张卡片最多依赖几张其他卡片。
const maxWaitsOn = 20

// normalizeWaitsOn 去掉重复的依赖，保留首次出现的顺序，并检查数量上限和 id 的取值。
// 返回的是新切片，不会改动传入的参数；结果永远不是 nil，这样存储和 JSON 里都是 []。
func normalizeWaitsOn(ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, invalidf("依赖的卡片 id 必须是正整数")
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) > maxWaitsOn {
		return nil, invalidf("一张卡片最多依赖 %d 张卡片", maxWaitsOn)
	}
	return out, nil
}

// validateWaitsOn 校验一张卡片的新依赖列表是否可以保存。cardID 是这张卡片自己，新建时传 0。
// boardCards 是同一看板的全部卡片，依赖必须都在其中（也就是同一看板），并且不能指向自己；
// 还要保证不会成环：从每个新依赖出发沿依赖链往下走，如果能走回 cardID，就说明 A 等 B、B 等 A，
// 两张卡都永远不会被领取。搜索在内存里完成，复杂度是看板卡片数加依赖边数的线性级。
func validateWaitsOn(cardID int64, deps []int64, boardCards []*Card) error {
	graph := make(map[int64][]int64, len(boardCards))
	for _, c := range boardCards {
		graph[c.ID] = c.WaitsOn
	}
	for _, dep := range deps {
		if dep == cardID {
			return invalidf("卡片不能依赖它自己")
		}
		if _, ok := graph[dep]; !ok {
			return invalidf("依赖的卡片 %d 不存在，或者不属于这个看板", dep)
		}
	}
	if cardID != 0 && reachesTarget(graph, deps, cardID) {
		return invalidf("依赖关系不能成环：被依赖的卡片直接或间接在等待这张卡片")
	}
	return nil
}

// reachesTarget 判断从 starts 出发，沿依赖链能不能走到 target。迭代的深度优先搜索，避免递归过深。
func reachesTarget(graph map[int64][]int64, starts []int64, target int64) bool {
	visited := make(map[int64]bool, len(graph))
	stack := append([]int64(nil), starts...)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == target {
			return true
		}
		if visited[id] {
			continue
		}
		visited[id] = true
		stack = append(stack, graph[id]...)
	}
	return false
}

// unfinishedDeps 返回卡片还没完成的依赖：依赖对应的卡片存在，并且所在的列不是完成列。
// 依赖的卡片已经被删除时不算，因为删除卡片时会把它从别的卡片的依赖里移除，
// 这里再忽略一次，是为了读到半途数据时也不会永远显示等待中。
// byID 是同一看板全部卡片按主键的索引，doneCols 是完成列的主键集合。
func unfinishedDeps(card *Card, byID map[int64]*Card, doneCols map[int64]bool) []*Card {
	var out []*Card
	for _, id := range card.WaitsOn {
		if dep, ok := byID[id]; ok && !doneCols[dep.ColumnID] {
			out = append(out, dep)
		}
	}
	return out
}
