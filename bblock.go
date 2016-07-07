package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/qeedquan/go-media/sdl"
	"github.com/qeedquan/go-media/sdl/sdlgfx"
	"github.com/qeedquan/go-media/sdl/sdlimage/sdlcolor"
	"github.com/qeedquan/go-media/sdl/sdlmixer"
	"github.com/qeedquan/go-media/sdl/sdlttf"
)

const (
	MaxScore = 100000
)

type State interface {
	Event()
	Update()
	Draw()
}

var (
	conf struct {
		width      int
		height     int
		difficulty int
		assets     string
		pref       string
		fullscreen bool
		sound      bool
		invincible bool
	}

	window   *sdl.Window
	renderer *sdl.Renderer
	texture  *sdl.Texture
	canvas   *image.RGBA
	surface  *sdl.Surface
	fps      sdlgfx.FPSManager

	state     State
	title     Title
	game      Game
	highscore uint64

	sfont *sdlttf.Font
	font  *sdlttf.Font
	lfont *sdlttf.Font

	click *sdlmixer.Chunk
	begin *sdlmixer.Chunk
	end   *sdlmixer.Chunk
)

func main() {
	runtime.LockOSThread()
	parseFlags()
	initSDL()
	loadAssets()

	title.Reset()
	state = &title
	for {
		state.Event()
		state.Update()

		renderer.SetDrawColor(sdlcolor.Black)
		renderer.Clear()
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.White}, image.ZP, draw.Src)
		state.Draw()
		texture.Update(nil, canvas.Pix, canvas.Stride)
		renderer.Copy(texture, nil, nil)
		renderer.Present()

		fps.Delay()
	}
}

func parseFlags() {
	conf.width = 50*4 + 3
	conf.height = 300
	conf.difficulty = 1
	conf.assets = filepath.Join(sdl.GetBasePath(), "assets")
	conf.pref = sdl.GetPrefPath("", "bblock")
	flag.StringVar(&conf.pref, "pref", conf.pref, "pref directory")
	flag.StringVar(&conf.assets, "assets", conf.assets, "assets directory")
	flag.IntVar(&conf.difficulty, "difficulty", conf.difficulty, "difficulty [1-5] (easiest to hardest)")
	flag.BoolVar(&conf.fullscreen, "fullscreen", false, "fullscreen mode")
	flag.BoolVar(&conf.sound, "sound", true, "enable sound")
	flag.BoolVar(&conf.invincible, "invincible", false, "be invincible")
	flag.Parse()

	if !(1 <= conf.difficulty && conf.difficulty <= 5) {
		ck(fmt.Errorf("Invalid difficulty: %v", conf.difficulty))
	}
}

func initSDL() {
	err := sdl.Init(sdl.INIT_EVERYTHING &^ sdl.INIT_AUDIO)
	ck(err)

	err = sdlttf.Init()
	ck(err)

	err = sdl.InitSubSystem(sdl.INIT_AUDIO)
	ek(err)

	err = sdlmixer.OpenAudio(44100, sdl.AUDIO_S16, 2, 8192)
	ek(err)

	_, err = sdlmixer.Init(sdlmixer.INIT_OGG)
	ek(err)

	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "best")

	wflag := sdl.WINDOW_RESIZABLE
	if conf.fullscreen {
		wflag |= sdl.WINDOW_FULLSCREEN_DESKTOP
	}
	window, renderer, err = sdl.CreateWindowAndRenderer(conf.width, conf.height, wflag)
	ck(err)

	renderer.SetLogicalSize(conf.width, conf.height)
	window.SetTitle("bblock")

	texture, err = renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, conf.width, conf.height)
	ck(err)

	canvas = image.NewRGBA(image.Rect(0, 0, conf.width, conf.height))

	surface, err = sdl.CreateRGBSurface(sdl.SWSURFACE, conf.width, conf.height, 32, 0x00FF0000, 0x0000FF00, 0x000000FF, 0xFF000000)
	ck(err)

	fps.Init()
	fps.SetRate(60)
}

func loadAssets() {
	sfont = loadFont(12)
	font = loadFont(18)
	lfont = loadFont(24)

	click = loadSound("click.ogg")
	begin = loadSound("begin.ogg")
	end = loadSound("end.ogg")
}

func loadFont(ptSize int) *sdlttf.Font {
	name := filepath.Join(conf.assets, "times.ttf")
	font, err := sdlttf.OpenFont(name, ptSize)
	ck(err)
	return font
}

func loadSound(name string) *sdlmixer.Chunk {
	name = filepath.Join(conf.assets, name)
	chunk, err := sdlmixer.LoadWAV(name)
	if ek(err) {
		return nil
	}
	return chunk
}

func playSound(sound *sdlmixer.Chunk) {
	if !conf.sound || sound == nil {
		return
	}
	sound.PlayChannel(-1, 0)
}

func loadScore() {
	name := filepath.Join(conf.pref, "hs.txt")
	f, err := os.Open(name)
	if ek(err) {
		return
	}
	defer f.Close()

	highscore = 0
	_, err = fmt.Fscan(f, &highscore)
	if ek(err) {
		highscore = 0
	}
	if highscore > MaxScore {
		highscore = MaxScore
	}
}

func saveScore() {
	name := filepath.Join(conf.pref, "hs.txt")
	f, err := os.Create(name)
	if ek(err) {
		return
	}
	_, err = fmt.Fprint(f, highscore)
	ek(err)

	err = f.Close()
	ek(err)
}

func ck(err error) {
	if err != nil {
		sdl.LogCritical(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
		sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "Error", err.Error(), window)
		os.Exit(1)
	}
}

func ek(err error) bool {
	if err != nil {
		sdl.LogError(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
		return true
	}
	return false
}

func printf(font *sdlttf.Font, fg sdl.Color, x, y int, format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	r, err := font.RenderUTF8BlendedEx(surface, text, fg)
	ck(err)
	draw.Draw(canvas, image.Rect(x, y, x+int(r.W), y+int(r.H)), surface, image.ZP, draw.Over)
}

func vline(x, y1, y2 int) {
	for ; y1 < y2; y1++ {
		canvas.Set(x, y1, sdlcolor.Red)
	}
}

type Title struct {
	blink int
}

func (c *Title) Reset() {
	c.blink = -30
	loadScore()
}

func (c *Title) Event() {
	for {
		ev := sdl.PollEvent()
		if ev == nil {
			break
		}
		switch ev := ev.(type) {
		case sdl.QuitEvent:
			os.Exit(0)
		case sdl.KeyDownEvent:
			switch ev.Sym {
			case sdl.K_ESCAPE:
				os.Exit(0)
			case sdl.K_RALT, sdl.K_LALT:
			default:
				game.Reset()
				state = &game
			}
		case sdl.MouseButtonDownEvent:
			game.Reset()
			state = &game
		}
	}
}

func (c *Title) Update() {}

func (c *Title) Draw() {
	printf(lfont, sdlcolor.Red, 65, 80, "bblock")
	if c.blink < 0 {
		printf(font, sdlcolor.Red, 30, 150, "PRESS ANY KEY")
	}
	if c.blink++; c.blink == 30 {
		c.blink = -30
	}

	printf(font, sdlcolor.Red, 35, 260, "HIGH SCORE %v", highscore)
}

type Game struct {
	score  uint64
	blocks []Block
	rate   float64
	count  int
	width  int
	height int
}

func (c *Game) Reset() {
	rand.Seed(time.Now().UnixNano())
	playSound(begin)
	c.score = 0
	c.blocks = c.blocks[:0]
	c.count = 0

	c.width = 50
	switch conf.difficulty {
	default:
	case 1:
		c.height = 100
		c.rate = 2
	case 2:
		c.height = 100
		c.rate = 3
	case 3:
		c.height = 100
		c.rate = 4
	case 4:
		c.height = 90
		c.rate = 4.5
	case 5:
		c.height = 80
		c.rate = 6
	}
}

func (c *Game) Event() {
	for {
		ev := sdl.PollEvent()
		if ev == nil {
			break
		}
		switch ev := ev.(type) {
		case sdl.QuitEvent:
			os.Exit(0)
		case sdl.KeyDownEvent:
			switch ev.Sym {
			case sdl.K_ESCAPE:
				os.Exit(0)
			}
		case sdl.MouseButtonDownEvent:
			switch ev.Button {
			case 1:
				c.clicked(ev)
			case 3:
				conf.invincible = !conf.invincible
				sdl.Log("Invinciblity toggle: %v", conf.invincible)
			}
		}
	}
}

func (c *Game) clicked(ev sdl.MouseButtonDownEvent) {
	p := image.Pt(int(ev.X), conf.height-int(ev.Y))
	for i, b := range c.blocks {
		if !(b.X <= p.X && p.X <= b.X+51) {
			continue
		}
		if !(b.Bottom <= float64(p.Y) && float64(p.Y) <= b.Bottom+float64(c.height)) {
			continue
		}
		playSound(click)
		if c.score < MaxScore {
			if c.score++; c.score >= highscore {
				highscore = c.score
			}
		}

		n := len(c.blocks) - 1
		c.blocks[i], c.blocks = c.blocks[n], c.blocks[:n]
	}
}

func (c *Game) Update() {
	c.updateBlocks()

	var u []Block
	for i := range c.blocks {
		b := &c.blocks[i]
		b.Move(c.rate)
		if b.Bottom <= c.rate {
			if !conf.invincible {
				playSound(end)
				title.Reset()
				saveScore()
				state = &title
				return
			}
		} else {
			u = append(u, *b)
		}
	}
	c.blocks = u
}

func (c *Game) updateBlocks() {
	if c.count++; c.count < int(float64(c.height)/c.rate) {
		return
	}
	c.count = 0

	var lanes, used []int
	for i := 0; i < 4; i++ {
		lanes = append(lanes, rand.Intn(4))
		if rand.Float64() < 0.75 {
			break
		}
	}

loop:
	for _, p := range lanes {
		for _, q := range used {
			if p == q {
				continue loop
			}
		}

		c.blocks = append(c.blocks, newBlock(p))
		used = append(used, p)
	}
}

func (c *Game) Draw() {
	xs := []int{50, 50*2 + 1, 50*3 + 2}
	for _, x := range xs {
		vline(x, 0, conf.height-25)
	}

	for _, b := range c.blocks {
		b.Draw(c.width, c.height)
	}

	printf(sfont, sdlcolor.Blue, 5, conf.height-20, "Current: %v", c.score)
	printf(sfont, sdlcolor.Blue, 150, conf.height-20, "Best: %v", highscore)
}

type Block struct {
	X, Col int
	Bottom float64
}

func newBlock(col int) Block {
	return Block{
		X:      col * 51,
		Col:    col,
		Bottom: float64(conf.height),
	}
}

func (b *Block) Move(rate float64) {
	b.Bottom -= rate
}

func (b *Block) Draw(w, h int) {
	x := b.X
	y := conf.height - int(b.Bottom) - w
	r := image.Rect(x, y, x+w, y+h)
	draw.Draw(canvas, r, image.NewUniform(color.Black), image.ZP, draw.Src)

	pink := image.NewUniform(sdl.Color{0xff, 0, 0xff, 0xff})
	for u := x; u < x+w; u++ {
		canvas.Set(u, y, pink)
		canvas.Set(u, y+h, pink)
	}
	for v := y; v < y+h; v++ {
		canvas.Set(x, v, pink)
		canvas.Set(x+w, v, pink)
	}
}
