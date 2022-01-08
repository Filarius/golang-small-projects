package main

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	_ "image/png"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}



type Core struct {
	data []float32
	w int
	h int
}



func (c *Core)  Init(w,h int) {
	c.data = make([]float32, w*h)
	c.w = w
	c.h = h
}

func (c *Core) Apply (img []byte,w,h int) []byte {

	wc,hc := c.w, c.h

	//b := make([]byte,w*h*4)
	bout:= make([]byte,w*h*4)

	for i:=0;i<w-wc;i++ {
		for j := 0; j < h-hc; j++ {
			x := (j*w + i)*4
			for z:=0;z<4;z++ {
				if z==3{
					bout[x+z] = 255
					continue
				}
				var  s float32 = 0
				for i2:=0;i2<wc;i2++{
					for j2:=0;j2<hc;j2++ {
						s += float32(img[x+z+(j2*w+i2)*4])*c.data[c.w*j2+i2]
					}
				}
				if s > 255{
					s = 255
				}else
				if s < 0 {
					s = 0
				}

				bout[x+z] = byte(s)
			}
		}
	}
	return bout
}
func (c *Core) ScoreComp (img1,img2 []byte,w,h int) uint64{
	var s uint64 = 0

	for i:=0;i<w-c.w;i++ {
		for j := 0; j < h-c.h; j++ {
			for z:=0;z<3;z++{ // no alpha channel
				q := w*j+i*4+z
				if img1[q]>img2[q]{
					s+= uint64(img1[q]-img2[q])
				}else{
					s+= uint64(img2[q]-img1[q])
				}

			}
			/*
			q,w,e,r := img1.At(i,j).RGBA()
			z,x,c,v := img2.At(i,j).RGBA()

			s+= uint64(math.Abs(float64(w-x)))
			s+= uint64(math.Abs(float64(e-c)))
			s+= uint64(math.Abs(float64(r-v)))
			*/
		}
	}
	return s
}
func (c *Core) Mutate(rate,value  float32){
	cnt := int(float32(len(c.data))*rate)
	for i:=0;i<cnt;i++ {
		n := rand.Intn(len(c.data))
		if rand.Intn(2) == 0 {
			c.data[n] += value
		} else {
			c.data[n] -= value
		}
	}
}

func (c *Core) Mating(core *Core) *Core{
	newcore := Core{}
	newcore.Init(c.w,c.h)
	for i:=0;i<len(core.data);i++{
		if rand.Intn(2)==0{
			newcore.data[i] = c.data[i]
		}else
		{
			newcore.data[i] = core.data[i]
		}
	}
	return &newcore
}

type CorePool struct {
	cores []*Core
	refcore *Core
	refimage []byte
	imw int
	imh int
	corecnt int
	origin_arr []byte
	mutrate float32
	mutvalue float32
	matingrate float32
}

func (p *CorePool) Init(corecount,w,h int,refimage []byte,imagew,imageh int, refcore *Core) {
	p.mutrate = 2.0/float32(len(refcore.data))
	p.mutvalue = 2*0.5*0.5/255.0
	p.matingrate = 2
	p.refimage = refimage
	p.imw = imagew
	p.imh = imageh
	p.refcore = refcore
	p.corecnt = corecount*0 +20
	p.cores = make([]*Core, corecount)
	for i:=0; i<len(p.cores);i++{
		p.cores[i] = &Core{}
		p.cores[i].Init(10,8)

		for j:=0; j<len(p.cores[i].data); j++{
			p.cores[i].data[j] = (1)/float32(len(p.cores[i].data))
		}



	}
}

func (p *CorePool) CalcScores() []uint64 {
	scores := make([]uint64,len(p.cores))
	//w,h := p.refcore.w, p.refcore.h
	wg := sync.WaitGroup{}
	ch := make(chan byte,16)
	for i:=0;i<len(p.cores);i++{
		wg.Add(1)
		ch <-1
		go func(i int) {
			a := p.cores[i].Apply(p.refimage, p.imw, p.imh)
			a = p.refcore.Apply(a, p.imw, p.imh)
			scores[i] = p.refcore.ScoreComp(a, p.refimage, p.imw, p.imh)
			<-ch
			wg.Done()
		}(i)
	}
	wg.Wait()
	return scores
}

func (p *CorePool) Update()  {
	mutatedcores :=make([]*Core,len(p.cores)/4)
	for i:=0; i<len(mutatedcores); i++ {
			mutatedcores[i] = &Core{}
			mutatedcores[i].Init(p.cores[0].w, p.cores[0].h)
			copy(mutatedcores[i].data, p.cores[0].data)
			mutatedcores[i].Mutate(p.mutrate, p.mutvalue)

	}
	p.cores = append(p.cores,mutatedcores...)

	nmax := len(p.cores)
	matecnt := int( float32(nmax) * p.matingrate )
	matedcores := make([]*Core,matecnt)
	for i:=0; i<matecnt; i++{
		x := 0
		y := 0
		for {
			x = rand.Intn(nmax)
			y = rand.Intn(nmax)
			if x!=y {break}
		}
			matedcores[i] = p.cores[x].Mating(p.cores[y])
		//println(1,i,matedcores[i]==nil)
	}
/*
	for i:=0;i<len(matedcores);i++{
		matedcores[i].Mutate(p.mutrate, p.mutvalue)
	}
*/

	{
		a :=len(p.cores)
		b :=len(matedcores)
		newcores := make([]*Core,a+b)
		for i:=0;i<a;i++{
			//println(2,i,p.cores[i]==nil)
			newcores[i] = p.cores[i]
		}
		for i:=0;i<b;i++{
			//println(3,i,matedcores[i]==nil)
			newcores[a+i] = matedcores[i]
		}
		p.cores = newcores
	}

	//p.cores = append(p.cores,matedcores...)
	/*
	for i,v := range p.cores{
		println(4,i,v==nil)
	}

	 */

	///////random image
	/*
	p.imw = p.refcore.w*10
	p.imh = p.refcore.h*10
	testimage := make([]byte,p.imw*p.imh*4)
	for i:=0;i<len(testimage);i++{
		testimage[i] = byte(rand.Intn(20)+100)
	}
	p.refimage = testimage
*/

	scores := p.CalcScores()
	smap := make([]int,len(scores))
	for i:=0;i<len(scores);i++{
		smap[i]=i
	}
	sort.SliceStable(smap,func(i,j int)bool{
		return (scores[smap[i]] < scores[smap[j]])
	})
	println(scores[0],scores[len(scores)-1])
	newcores := make([]*Core,p.corecnt)
	for i:=0; i<p.corecnt; i++{
		newcores[i] = p.cores[smap[i]]
	}
	p.cores = newcores
}

func (p *CorePool) GetBest() *Core{
	return p.cores[0]
}

func (p *CorePool) Blur() {
	for _,v := range p.cores {
		d := v.data
		for i:=0;i<v.w-1;i+=2{
			for j:=0;j<v.h-1;j+=2{
				d[j*v.w + i] = (d[j*v.w + i] + d[(j+1)*v.w + i] + d[j*v.w + i+1] + d[(j+1)*v.w + i+1])/4
				d[j*v.w + i+1] = d[j*v.w + i]
				d[(j+1)*v.w + i] = d[j*v.w + i]
				d[(j+1)*v.w + i+1] = d[j*v.w + i]
			}
		}
	}

}


func Image2Array(m *ebiten.Image) []byte{
	w,h := m.Size()
	arr := make([]byte,w*h*4)
	for i:=0;i<w;i++{
		for j:=0;j<h;j++{
			x:= (j*w+i)*4
			a1,a2,a3,a4 := m.At(i,j).RGBA()
			arr[x+0] = byte(a1)
			arr[x+1] = byte(a2)
			arr[x+2] = byte(a3)
			arr[x+3] = byte(a4)
		}
	}
	return arr
}


func Image2CoreArray(m *ebiten.Image) []float32{
	w,h := m.Size()
	arr := make([]byte,w*h)
	var sum uint64 = 0
	for i:=0;i<w;i++{
		for j:=0;j<h;j++{
			x:= (j*w+i)
			a1,_,_,_ := m.At(i,j).RGBA()
			sum += uint64(a1&0xff)
			arr[x+0] = byte(a1)
		}
	}
	f:= float32(sum)
	fa := make([]float32,len(arr))
	for i:=0; i<len(arr) ;i++{
		fa[i] = float32(arr[i])/f
	}

	return fa
}

func CoreArray2ImageArray(c []float32,w,h int) []byte{
  var min float32 = 10.0
  var max float32  = -10.0
  a := make([]byte,w*h*4)
  for _,v := range c{
  	if min > v{
  		min = v
	}
	if max < v {
		max = v
	}
  }
	for i,v := range c {
		//val := 255-2*255*(v+min)/(max-min)
		val := v * 255* 2
		if val > 255 {
			val = 255
		}
		if val < -255{
			val = -255
		}
		if val < 0 {
			a[i*4+0] = byte(-val)
			a[i*4+1] = 0
			a[i*4+2] = 0
			a[i*4+3] = 255
		}else{
			a[i*4+0] = 0
			a[i*4+1] = byte(val)
			a[i*4+2] = 0
			a[i*4+3] = 255
		}
	}
	return a
}

const (
	screenWidth  = 800
	screenHeight = 800
)

type Game struct {

	pixels []byte
	img_origin *ebiten.Image
	origin []byte
	virgin []byte
	blurred []byte


	imgar []byte
	core Core
	uncore *Core
	canstart bool
	lastupdatetime time.Time
	mut float32
	corepool *CorePool
	t1 []byte
	t2 []byte
	t3 []byte

}

const mutagen float32 = 0.01

func (g *Game) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyNumpad9){
		g.corepool.mutvalue *= 1.01
	}
	if ebiten.IsKeyPressed(ebiten.KeyNumpad6){
		g.corepool.mutvalue *= 1/1.01
	}
	if g.img_origin == nil{
		g.canstart = false
		return nil
	}
	if !g.canstart{
		return nil
	}
	//g.corepool.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {

	if g.pixels == nil {
		g.pixels = make([]byte, screenWidth*screenHeight*4)
	}
	if g.img_origin == nil {
		g.canstart = false
		picture, _, _ := ebitenutil.NewImageFromFile("1.png")
		coreim, _, _ := ebitenutil.NewImageFromFile("core.png")
		coreblur := Core{}
		corepool := CorePool{}

		h := 0
		w := 0
		w, h = coreim.Size()
		coreblur.Init(w, h)
		coreblur.data = Image2CoreArray(coreim)
		w2, h2 := picture.Size()
		corepool.Init(20, w, h, Image2Array(picture), w2, h2, &coreblur)
		g.corepool = &corepool
		g.origin = Image2Array(picture)
		g.img_origin = picture

		copy(g.virgin, g.origin)

		g.core = coreblur

		g.blurred = coreblur.Apply(g.origin, w, h)
		go func() {
			for {
				g.corepool.Update()
				if ebiten.IsKeyPressed(ebiten.KeyEnter){
					g.corepool.Blur()
				}
				if ebiten.IsKeyPressed(ebiten.KeyNumpad7){
					for _,v := range g.corepool.cores{
						for i,q := range v.data{
							v.data[i] = q*0.99
						}
					}
				}
			}
		}()
		w, h = g.img_origin.Size()
		g.t1 = g.core.Apply(g.origin, w, h)
		g.canstart = true
	}
	img := ebiten.NewImageFromImage(g.img_origin)
	w, h := g.img_origin.Size()

	ops := &ebiten.DrawImageOptions{}
	ops.Filter = ebiten.FilterNearest

	img.ReplacePixels(g.corepool.GetBest().Apply(g.origin, w, h))
	//img.ReplacePixels(g.blurred)
	ops.GeoM.Scale(4, 4)
	ops.GeoM.Translate(100, 000)
	screen.DrawImage(img, ops) // untiblur  big

	ops.GeoM.Scale(0.25, 0.25)
	ops.GeoM.Translate(-20, 100)
	screen.DrawImage(img, ops) // untiblur small

	ops.GeoM.Translate(0, float64(g.img_origin.Bounds().Dy()))
	if g.t1 != nil {
		img.ReplacePixels(g.t1)
	}
	screen.DrawImage(img,ops) // blured small

	img.ReplacePixels(g.origin)
	ops.GeoM.Translate(0, float64(g.img_origin.Bounds().Dy()))
	screen.DrawImage(img,ops) // original

	img.ReplacePixels(g.core.Apply( g.corepool.GetBest().Apply(g.origin,w,h), w ,h ))
	ops.GeoM.Translate(0, float64(g.img_origin.Bounds().Dy()))
	screen.DrawImage(img,ops) // recovered original


	core := g.corepool.GetBest()
	img = ebiten.NewImage(core.w, core.h)
	img.ReplacePixels(CoreArray2ImageArray(core.data, core.w, core.h))
	ops = &ebiten.DrawImageOptions{}
	ops.GeoM.Scale(10,10)
	//ops.GeoM.Translate(0, float64(g.img_origin.Bounds().Dy())*6)
	ops.Filter = ebiten.FilterNearest
	screen.DrawImage(img,ops) // image of core

	//ops.GeoM.Translate(0, float64(g.img_origin.Bounds().Dy()))
	//ops.GeoM.Scale(10,10)
	//
	//screen.DrawImage(img,ops) // recovered original



	ebitenutil.DebugPrint(screen, fmt.Sprintf("                                 TPS: %0.2f  FPS: %0.2f MUT: %0.7f ", ebiten.CurrentTPS(),ebiten.CurrentFPS(),g.corepool.mutvalue))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	//ebiten.SetMaxTPS(60)

	//ebiten.SetFPSMode(ebiten.FPSModeVsyncOffMaximum)
	//ebiten.SetFPSMode(ebiten.FPSModeVsyncOffMaximum) ebiten.SetMaxTPS(500)
	//ebiten.SetMaxTPS(ebiten.SyncWithFPS)

	//ebiten.SetMaxTPS(5)
	//println(ebiten.ScreenSizeInFullscreen())

	g := &Game{

	}



	ebiten.SetWindowSize(800, 900)
	//ebiten.SetWindowResizable(true)
	ebiten.SetWindowTitle("Game of Life (Ebiten Demo)")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}