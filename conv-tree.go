package convtree

import (
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
)

type ConvTree struct {
	ID               string
	IsLeaf           bool
	MaxPoints        int
	MaxDepth         int
	Depth            int
	GridSize         int
	ConvNum          int
	Kernel           [][]float64
	Points           []Point
	MinXLength       float64
	MinYLength       float64
	TopLeft          Point
	BottomRight      Point
	ChildTopLeft     *ConvTree
	ChildTopRight    *ConvTree
	ChildBottomLeft  *ConvTree
	ChildBottomRight *ConvTree
	BaselineTags     []string
}

func NewConvTree(topLeft Point, bottomRight Point, minXLength float64, minYLength float64, maxPoints int, maxDepth int,
	convNumber int, gridSize int, kernel [][]float64, initPoints []Point) (ConvTree, error) {
	if topLeft.X >= bottomRight.X {
		err := errors.New("X of top left point is larger or equal to X of bottom right point")
		return ConvTree{}, err
	}
	if topLeft.Y <= bottomRight.Y {
		err := errors.New("Y of bottom right point is larger or equal to Y of top left point")
		return ConvTree{}, err
	}
	id := uuid.New().String()
	if !checkKernel(kernel) {
		kernel = [][]float64{
			[]float64{0.5, 0.5, 0.5},
			[]float64{0.5, 1.0, 0.5},
			[]float64{0.5, 0.5, 0.5},
		}
	}
	tree := ConvTree{
		IsLeaf:      true,
		ID:          id,
		MaxPoints:   maxPoints,
		GridSize:    gridSize,
		ConvNum:     convNumber,
		Kernel:      kernel,
		MaxDepth:    maxDepth,
		TopLeft:     topLeft,
		BottomRight: bottomRight,
		Points:      []Point{},
		MinXLength:  minXLength,
		MinYLength:  minYLength,
	}
	if initPoints != nil {
		tree.Points = initPoints
	}
	if tree.checkSplit() {
		tree.split()
	} else {
		tree.getBaseline()
	}
	return tree, nil
}

func checkKernel(kernel [][]float64) bool {
	if kernel == nil || len(kernel) == 0 {
		return false
	}
	if kernel[0] == nil {
		return false
	}
	xSize, ySize := len(kernel[0]), len(kernel)
	if xSize != ySize {
		return false
	}
	for _, row := range kernel {
		if len(row) != xSize {
			return false
		}
	}
	return true
}

func (tree *ConvTree) split() {
	xSize, ySize := tree.GridSize, tree.GridSize
	grid := make([][]float64, xSize)
	xStep := (tree.BottomRight.X - tree.TopLeft.X) / float64(xSize)
	yStep := (tree.TopLeft.Y - tree.BottomRight.Y) / float64(ySize)
	for i := 0; i < xSize; i++ {
		grid[i] = make([]float64, ySize)
		for j := 0; j < ySize; j++ {
			xLeft := tree.TopLeft.X + float64(i)*xStep
			xRight := tree.TopLeft.X + float64(i+1)*xStep
			yBottom := tree.BottomRight.Y + float64(j)*yStep
			yTop := tree.BottomRight.Y + float64(j+1)*yStep
			grid[i][j] = float64(tree.getNodeWeight(xLeft, xRight, yTop, yBottom))
		}
	}
	convolved := normalizeGrid(grid)
	for i := 0; i < tree.ConvNum; i++ {
		tmpGrid, err := convolve(convolved, tree.Kernel, 1, 1)
		if err != nil {
			fmt.Println(err)
			break
		}
		convolved = normalizeGrid(tmpGrid)
	}
	convolved = normalizeGrid(convolved)
	xMax, yMax := getSplitPoint(convolved)
	if xMax < 1 || xMax >= (len(convolved)-1) {
		xMax = len(convolved) / 2
	}
	if yMax < 1 || yMax >= (len(convolved[0])-1) {
		yMax = len(convolved[0]) / 2
	}
	xOffset := float64(xMax) * xStep
	yOffset := float64(yMax) * yStep

	xRight := tree.TopLeft.X + xOffset
	if xRight-tree.TopLeft.X < tree.MinXLength {
		xRight = tree.TopLeft.X + tree.MinXLength
	}
	if tree.BottomRight.X-xRight < tree.MinXLength {
		xRight = tree.BottomRight.X - tree.MinXLength
	}
	yBottom := tree.BottomRight.Y + yOffset
	if yBottom-tree.BottomRight.Y < tree.MinYLength {
		yBottom = tree.BottomRight.Y + tree.MinYLength
	}
	if tree.TopLeft.Y-yBottom < tree.MinYLength {
		yBottom = tree.TopLeft.Y - tree.MinYLength
	}
	id := uuid.New().String()
	tree.ChildTopLeft = &ConvTree{
		ID:      id,
		TopLeft: tree.TopLeft,
		BottomRight: Point{
			X: xRight,
			Y: yBottom,
		},
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildTopLeft.Points = tree.filterSplitPoints(tree.ChildTopLeft.TopLeft, tree.ChildTopLeft.BottomRight)
	if tree.ChildTopLeft.checkSplit() {
		tree.ChildTopLeft.split()
	} else {
		tree.ChildTopLeft.BaselineTags = tree.BaselineTags
		tree.ChildTopLeft.getBaseline()
	}

	id = uuid.New().String()
	tree.ChildTopRight = &ConvTree{
		ID: id,
		TopLeft: Point{
			X: xRight,
			Y: tree.TopLeft.Y,
		},
		BottomRight: Point{
			X: tree.BottomRight.X,
			Y: yBottom,
		},
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildTopRight.Points = tree.filterSplitPoints(tree.ChildTopRight.TopLeft, tree.ChildTopRight.BottomRight)
	if tree.ChildTopRight.checkSplit() {
		tree.ChildTopRight.split()
	} else {
		tree.ChildTopRight.BaselineTags = tree.BaselineTags
		tree.ChildTopRight.getBaseline()
	}

	id = uuid.New().String()
	tree.ChildBottomLeft = &ConvTree{
		ID: id,
		TopLeft: Point{
			X: tree.TopLeft.X,
			Y: yBottom,
		},
		BottomRight: Point{
			X: xRight,
			Y: tree.BottomRight.Y,
		},
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildBottomLeft.Points = tree.filterSplitPoints(tree.ChildBottomLeft.TopLeft, tree.ChildBottomLeft.BottomRight)
	if tree.ChildBottomLeft.checkSplit() {
		tree.ChildBottomLeft.split()
	} else {
		tree.ChildBottomLeft.BaselineTags = tree.BaselineTags
		tree.ChildBottomLeft.getBaseline()
	}

	id = uuid.New().String()
	tree.ChildBottomRight = &ConvTree{
		ID: id,
		TopLeft: Point{
			X: xRight,
			Y: yBottom,
		},
		BottomRight: tree.BottomRight,
		MaxPoints:   tree.MaxPoints,
		MaxDepth:    tree.MaxDepth,
		Kernel:      tree.Kernel,
		Depth:       tree.Depth + 1,
		GridSize:    tree.GridSize,
		ConvNum:     tree.ConvNum,
		MinXLength:  tree.MinXLength,
		MinYLength:  tree.MinYLength,
		IsLeaf:      true,
	}
	tree.ChildBottomRight.Points = tree.filterSplitPoints(tree.ChildBottomRight.TopLeft, tree.ChildBottomRight.BottomRight)
	if tree.ChildBottomRight.checkSplit() {
		tree.ChildBottomRight.split()
	} else {
		tree.ChildBottomRight.BaselineTags = tree.BaselineTags
		tree.ChildBottomRight.getBaseline()
	}

	tree.IsLeaf = false
	tree.Points = nil
}

func (tree *ConvTree) getBaseline() {
	tagValues := map[string]int{}
	for _, item := range tree.Points {
		if item.Content != nil {
			if tags, ok := item.Content.([]string); ok {
				itemTags := map[string]bool{}
				for _, tag := range tags {
					if _, ok := itemTags[tag]; !ok {
						itemTags[tag] = true
					}
				}
				for tag := range itemTags {
					if _, ok := tagValues[tag]; !ok {
						tagValues[tag] = 0
					}
					tagValues[tag]++
				}

			}
		}
	}
	if len(tagValues) > 0 {
		filteredTags := filterTags(tagValues)
		tree.BaselineTags = filteredTags
	}
}

func filterTags(tags map[string]int) []string {
	numbers := make([]float64, len(tags))
	i := 0
	for _, v := range tags {
		numbers[i] = float64(v)
		i++
	}
	avg := mean(numbers)
	splitValue := int(avg)
	result := []string{}
	for k, v := range tags {
		if v > splitValue {
			result = append(result, k)
		}
	}
	return result
}

func getSplitPoint(grid [][]float64) (int, int) {
	threshold := 0.8
	maxX, maxY := 0, 0
	maxValue := 0.0
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			if grid[i][j] > maxValue {
				maxValue = grid[i][j]
				maxX, maxY = i, j
			}
		}
	}
	splitValue := maxValue * threshold
	counter := 1
	itemFound := false
	splitX, splitY := 0, 0
	for {
		x, y := 0, 0
		vals := []float64{}
		itemFound = false
		i := maxX - counter
		if i >= 0 {
			for j := maxY - counter; j <= maxY+counter; j++ {
				if j >= 0 && j < len(grid[0]) {
					if grid[i][j] > splitValue {
						itemFound = true
						x = i
						vals = append(vals, grid[i][j])
					}
				}
			}
		}
		i = maxX + counter
		if i < len(grid) {
			for j := maxY - counter; j <= maxY+counter; j++ {
				if j >= 0 && j < len(grid[0]) {
					if grid[i][j] > splitValue {
						itemFound = true
						if math.Abs(float64(x-len(grid)/2)) > math.Abs(float64(i-len(grid)/2)) {
							x = i
						}
						vals = append(vals, grid[i][j])
					}
				}
			}
		}
		i = maxY - counter
		if i >= 0 {
			for j := maxX - counter; j <= maxX+counter; j++ {
				if j >= 0 && j < len(grid) {
					if grid[j][i] > splitValue {
						itemFound = true
						y = i
						if j != maxX-counter && j != maxX+counter {
							vals = append(vals, grid[j][i])
						}
					}
				}
			}
		}
		i = maxY + counter
		if i < len(grid[0]) {
			for j := maxX - counter; j <= maxX+counter; j++ {
				if j >= 0 && j < len(grid) {
					if grid[j][i] > splitValue {
						itemFound = true
						if math.Abs(float64(y-len(grid[0])/2)) > math.Abs(float64(i-len(grid[0])/2)) {
							y = i
						}
						if j != maxX-counter && j != maxX+counter {
							vals = append(vals, grid[j][i])
						}
					}
				}
			}
		}
		if !itemFound {
			break
		}
		if x != 0 {
			splitX = x
		}
		if y != 0 {
			splitY = y
		}
		splitValue = mean(vals) * threshold
		counter++
	}
	if splitX > maxX {
		splitX++
	} else {
		splitX--
	}
	if splitY > maxY {
		splitY++
	} else {
		splitY--
	}
	return splitX, splitY
}

func mean(in []float64) float64 {
	sum := 0.0
	for _, v := range in {
		sum += v
	}
	return sum / float64(len(in))
}

func (tree *ConvTree) Insert(point Point, allowSplit bool) {
	if !tree.IsLeaf {
		if point.X >= tree.ChildTopLeft.TopLeft.X && point.X <= tree.ChildTopLeft.BottomRight.X &&
			point.Y <= tree.ChildTopLeft.TopLeft.Y && point.Y >= tree.ChildTopLeft.BottomRight.Y {
			tree.ChildTopLeft.Insert(point, allowSplit)
			return
		}
		if point.X >= tree.ChildTopRight.TopLeft.X && point.X <= tree.ChildTopRight.BottomRight.X &&
			point.Y <= tree.ChildTopRight.TopLeft.Y && point.Y >= tree.ChildTopRight.BottomRight.Y {
			tree.ChildTopRight.Insert(point, allowSplit)
			return
		}
		if point.X >= tree.ChildBottomLeft.TopLeft.X && point.X <= tree.ChildBottomLeft.BottomRight.X &&
			point.Y <= tree.ChildBottomLeft.TopLeft.Y && point.Y >= tree.ChildBottomLeft.BottomRight.Y {
			tree.ChildBottomLeft.Insert(point, allowSplit)
			return
		}
		if point.X >= tree.ChildBottomRight.TopLeft.X && point.X <= tree.ChildBottomRight.BottomRight.X &&
			point.Y <= tree.ChildBottomRight.TopLeft.Y && point.Y >= tree.ChildBottomRight.BottomRight.Y {
			tree.ChildBottomRight.Insert(point, allowSplit)
			return
		}
	} else {
		tree.Points = append(tree.Points, point)
		if allowSplit {
			if tree.checkSplit() {
				tree.split()
			}
		}
	}
}

func (tree *ConvTree) Check() {
	if tree.checkSplit() {
		tree.split()
	}
}

func (tree *ConvTree) Clear() {
	tree.Points = nil
	if tree.ChildBottomLeft != nil {
		tree.ChildBottomLeft.Clear()
	}
	if tree.ChildBottomRight != nil {
		tree.ChildBottomRight.Clear()
	}
	if tree.ChildTopLeft != nil {
		tree.ChildTopLeft.Clear()
	}
	if tree.ChildTopRight != nil {
		tree.ChildTopRight.Clear()
	}
}

func (tree *ConvTree) checkSplit() bool {
	cond1 := (tree.BottomRight.X-tree.TopLeft.X) > 2*tree.MinXLength && (tree.TopLeft.Y-tree.BottomRight.Y) > 2*tree.MinYLength
	totalWeight := 0
	for _, point := range tree.Points {
		totalWeight += point.Weight
	}
	cond2 := totalWeight > tree.MaxPoints && tree.Depth < tree.MaxDepth
	return cond1 && cond2
}

func (tree *ConvTree) getNodeWeight(xLeft, xRight, yTop, yBottom float64) int {
	total := 0
	for _, point := range tree.Points {
		if point.X >= xLeft && point.X <= xRight && point.Y >= yBottom && point.Y <= yTop {
			total += point.Weight
		}
	}
	return total
}

func (tree *ConvTree) filterSplitPoints(topLeft, bottomRight Point) []Point {
	result := []Point{}
	for _, point := range tree.Points {
		if point.X >= topLeft.X && point.X <= bottomRight.X && point.Y >= bottomRight.Y && point.Y <= topLeft.Y {
			result = append(result, point)
		}
	}
	return result
}

func convolve(grid [][]float64, kernel [][]float64, stride, padding int) ([][]float64, error) {
	if stride < 1 {
		err := errors.New("convolutional stride must be larger than 0")
		return nil, err
	}
	if padding < 1 {
		err := errors.New("convolutional padding must be larger than 0")
		return nil, err
	}
	kernelSize := len(kernel)
	if len(grid) < kernelSize {
		err := errors.New("grid width is less than convolutional kernel size")
		return nil, err
	}
	if len(grid[0]) < kernelSize {
		err := errors.New("grid height is less than convolutional kernel size")
		return nil, err
	}
	procGrid := make([][]float64, len(grid)+2*padding)
	for i := 0; i < padding; i++ {
		procGrid[i] = make([]float64, len(grid)+2*padding)
		for j := range procGrid[i] {
			procGrid[i][j] = 0
		}
	}
	for i := 1; i < (len(procGrid) - 1); i++ {
		procGrid[i] = make([]float64, len(grid)+2*padding)
		procGrid[i][0] = 0
		for j := 1; j < len(procGrid[i])-1; j++ {
			procGrid[i][j] = grid[i-padding][j-padding]
		}
		procGrid[i][len(procGrid[i])-1] = 0
	}
	for i := 0; i < padding; i++ {
		procGrid[len(procGrid)-i-1] = make([]float64, len(grid)+2*padding)
		for j := range procGrid[len(procGrid)-i-1] {
			procGrid[len(procGrid)-i-1][j] = 0
		}
	}
	resultWidth := int((len(grid)-kernelSize+2*padding)/stride) + 1
	resultHeight := int((len(grid[0])-kernelSize+2*padding)/stride) + 1
	result := make([][]float64, resultWidth)
	for i := 0; i < resultWidth; i++ {
		result[i] = make([]float64, resultHeight)
		for j := 0; j < resultHeight; j++ {
			total := 0.0
			for x := 0; x < kernelSize; x++ {
				for y := 0; y < kernelSize; y++ {
					posX := stride*i + x
					posY := stride*j + y
					if posX >= 0 && posX < len(procGrid) && posY >= 0 && posY < len(procGrid[0]) {
						total += procGrid[posX][posY] * kernel[x][y]
					}
				}
			}
			result[i][j] = total
		}
	}
	return result, nil
}

func normalizeGrid(grid [][]float64) [][]float64 {
	maxValue := -math.MaxFloat64
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			if grid[i][j] > maxValue {
				maxValue = grid[i][j]
			}
		}
	}
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			grid[i][j] = grid[i][j] / maxValue
		}
	}
	return grid
}
