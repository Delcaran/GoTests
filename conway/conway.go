package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/rivo/tview"
)

type cell struct {
	x           int
	y           int
	alive       bool
	_id         int
	_neighbours int
}

type grid struct {
	size                int
	connectedEastWest   bool
	connectedNorthSouth bool
	population          [][]cell
}

// Initialize the grid
func (g *grid) initialize(population float32, size int, ew bool, ns bool) {
	g.size = size
	g.connectedEastWest = ew
	g.connectedNorthSouth = ns
	g.population = nil
	id := 0
	for x := 0; x < size; x++ {
		column := make([]cell, size, size)
		for y := 0; y < size; y++ {
			column[y] = cell{x, y, rand.Float32() < population, id, 0}
			id++
		}
		g.population = append(g.population, column)
	}
	for x := 0; x < g.size; x++ {
		for y := 0; y < g.size; y++ {
			g.countAliveNeighbours(&g.population[x][y])
		}
	}
}

// print a grid on screen
func (g grid) print(app *tview.Application, visualization *tview.TextView) {
	var buffer bytes.Buffer
	for x := 0; x < g.size; x++ {
		for y := 0; y < g.size; y++ {
			if g.population[x][y].alive {
				buffer.WriteString("#")
			} else {
				buffer.WriteString(" ")
			}
		}
		buffer.WriteString("\n")
	}
	visualization.SetText(buffer.String())
	app.ForceDraw()
}

func (g grid) getCell(x int, y int) *cell {
	if x < 0 {
		if g.connectedEastWest {
			x = g.size - 1
		} else {
			x = 0
		}
	}
	if x >= g.size {
		if g.connectedEastWest {
			x = 0
		} else {
			x = g.size - 1
		}
	}
	if y < 0 {
		if g.connectedNorthSouth {
			y = g.size - 1
		} else {
			y = 0
		}
	}
	if y >= g.size {
		if g.connectedNorthSouth {
			y = 0
		} else {
			y = g.size - 1
		}
	}
	return &g.population[x][y]
}

func (g grid) countAliveNeighbours(c *cell) {
	alive := 0
	neighbours := make([]*cell, 0, 0)
	for x := -1; x <= 1; x++ {
		for y := -1; y <= 1; y++ {
			parsing := g.getCell(c.x+x, c.y+y)
			if parsing._id != c._id {
				insert := true
				for _, element := range neighbours {
					if parsing._id == element._id {
						insert = false
						break
					}
				}
				if insert {
					neighbours = append(neighbours, parsing)
				}
			}
		}
	}
	for _, element := range neighbours {
		if element.alive {
			alive++
		}
	}
	c._neighbours = alive
	//c.peekEvolve()
}

func (c cell) peekEvolve() {
	if c.alive {
		if c._neighbours < 2 {
			fmt.Printf("%d will DIE by UNDERPOPULATION\n", c._id)
			return
		}
		if c._neighbours > 3 {
			fmt.Printf("%d will DIE by OVERPOPULATION\n", c._id)
			return
		}
		fmt.Printf("%d will STAY ALIVE\n", c._id)
	} else {
		if c._neighbours == 3 {
			fmt.Printf("%d WILL BORN\n", c._id)
		}
	}
}

func (c *cell) evolve() {
	if c.alive {
		if c._neighbours < 2 || c._neighbours > 3 {
			c.alive = false
			//fmt.Printf("%d DIES\n", c._id)
		}
	} else {
		if c._neighbours == 3 {
			//fmt.Printf("%d BORN\n", c._id)
			c.alive = true
		}
	}
}

func (g *grid) evolve() {
	for x := 0; x < g.size; x++ {
		for y := 0; y < g.size; y++ {
			g.population[x][y].evolve()
		}
	}
	for x := 0; x < g.size; x++ {
		for y := 0; y < g.size; y++ {
			g.countAliveNeighbours(&g.population[x][y])
		}
	}
}

func (g grid) equal(g2 grid) bool {
	for x := 0; x < g.size; x++ {
		for y := 0; y < g.size; y++ {
			if g.population[x][y].alive != g2.population[x][y].alive {
				return false
			}
		}
	}
	return true
}

func (g grid) copy() grid {
	var g2 grid
	g2.initialize(0.0, g.size, g.connectedEastWest, g.connectedNorthSouth)
	for x := 0; x < g.size; x++ {
		for y := 0; y < g.size; y++ {
			g2.population[x][y] = g.population[x][y]
		}
	}
	return g2
}

func isNumeric(v string, r rune) bool {
	if _, err := strconv.Atoi(v); err == nil {
		return true
	}
	return false
}

func run(app *tview.Application, visualization *tview.TextView, status *tview.TextView, population float32, size int, ew bool, ns bool) {
	generation := 0
	for run := true; run; {
		output := ""
		oldGrid := g.copy()
		g.evolve()
		generation++
		g.print(app, visualization)
		output = fmt.Sprintf("Generation %d", generation)
		status.SetText(output)
		app.Draw()
		if oldGrid.equal(g) {
			output = fmt.Sprintf("Generation %d\n\nGAME OVER", generation)
			status.SetText(output)
			app.ForceDraw()
			run = false
			continue
		}
		time.Sleep(1 * time.Second)
	}
}

func ready(app *tview.Application, visualization *tview.TextView, g *grid, population float32, size int, ew bool, ns bool) {
	rand.Seed(time.Now().UnixNano())
	g.initialize(population/100, size, ew, ns)
	g.print(app, visualization)
}

var population float32
var size int
var ew bool
var ns bool
var g grid

func setPopulation(v string) {
	if intpop, err := strconv.Atoi(v); err == nil {
		population = float32(intpop)
	}
}

func setSize(v string) {
	if val, err := strconv.Atoi(v); err == nil {
		size = val
	}
}

func setEW(v bool) {
	ew = v
}

func setNS(v bool) {
	ns = v
}

func main() {
	app := tview.NewApplication()
	setSize("20")
	setPopulation("30")
	setEW(false)
	setNS(false)

	visualization := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false).
		SetTextAlign(tview.AlignCenter).
		SetChangedFunc(func() {
			app.Draw()

		})
	visualization.SetBorder(true).SetTitle("WORLD").SetTitleAlign(tview.AlignCenter)
	ready(app, visualization, &g, population, size, ew, ns)

	status := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false).
		SetChangedFunc(func() {
			app.Draw()
		})
	status.SetBorder(true).SetTitle("Status").SetTitleAlign(tview.AlignCenter)

	configuration := tview.NewForm().
		AddInputField("Size", "20", 0, isNumeric, func(v string) {
			setSize(v)
			ready(app, visualization, &g, population, size, ew, ns)
		}).
		AddInputField("% populated", "30", 0, isNumeric, func(v string) {
			setPopulation(v)
			ready(app, visualization, &g, population, size, ew, ns)
		}).
		AddCheckbox("Connected East-West", false, func(v bool) {
			setEW(v)
			ready(app, visualization, &g, population, size, ew, ns)
		}).
		AddCheckbox("Connected North-South", false, func(v bool) {
			setNS(v)
			ready(app, visualization, &g, population, size, ew, ns)
		}).
		AddButton("Reseed", func() { ready(app, visualization, &g, population, size, ew, ns) }).
		AddButton("Start", func() { run(app, visualization, status, population, size, ew, ns) }).
		AddButton("Quit", func() { app.Stop() })
	configuration.SetBorder(true).SetTitle("Configuration").SetTitleAlign(tview.AlignCenter)

	flex := tview.NewFlex().
		AddItem(configuration, 0, 1, true).
		AddItem(visualization, 0, 5, true).
		AddItem(status, 0, 1, true)

	if err := app.SetRoot(flex, true).SetFocus(configuration).Run(); err != nil {
		panic(err)
	}

}
