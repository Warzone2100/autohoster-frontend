package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	mapsdatabase "github.com/maxsupermanhd/go-wz/maps-database"
	"github.com/maxsupermanhd/go-wz/packet"
	"github.com/maxsupermanhd/go-wz/replay"
	"golang.org/x/image/draw"
)

func getReplayStuffs(gid int) (rpl *replay.Replay, mapimg image.Image, err error) {
	log.Println("Getting replay from storage...")
	replaycontent, err := getReplayFromStorage(gid)
	if err != nil {
		if err == errReplayNotFound {
			return
		}
		return
	}
	log.Println("Loading replay...")
	rpl, err = replay.ReadReplay(bytes.NewBuffer(replaycontent))
	if err != nil {
		return
	}
	if rpl == nil {
		return nil, nil, errors.New("replay is nil")
	}
	log.Println("Fetching map hash...")
	maphash := ""
	err = dbpool.QueryRow(context.Background(), `SELECT map_hash FROM games WHERE id = $1`, gid).Scan(&maphash)
	if err != nil {
		return
	}
	log.Println("Fetching map image...")
	mapimg, err = mapsdatabase.FetchMapPreview(maphash)
	if err != nil {
		return
	}
	log.Println("Replay stuffs prepared")
	return
}

func APIgetReplayHeatmap(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gids := params["gid"]
	gid, err := strconv.Atoi(gids)
	if err != nil {
		return 400, nil
	}

	rpl, mapimg, err := getReplayStuffs(gid)
	if err != nil {
		if err == errReplayNotFound {
			return 204, nil
		}
		return 500, err
	}

	img, err := genReplayHeatmap(*rpl, mapimg)
	if err != nil {
		return 500, err
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "image/png")
	w.Write(img)
	return -1, nil
}

func genReplayHeatmap(rpl replay.Replay, mapimg image.Image) ([]byte, error) {
	const scale = 16
	const dotsize = 18

	const mapimgscale = scale

	img := image.NewRGBA(image.Rectangle{Max: mapimg.Bounds().Max.Mul(mapimgscale)})
	draw.NearestNeighbor.Scale(img, img.Rect, mapimg, mapimg.Bounds(), draw.Src, nil)

	dots := make([]draw.Image, len(rpl.Settings.GameOptions.NetplayPlayers))
	for i, v := range rpl.Settings.GameOptions.NetplayPlayers {
		if v.Colour < 0 || v.Colour >= len(playerColors) {
			log.Printf("Color overflow: %#v", v)
		} else {
			dots[i] = mkDot(dotsize, playerColors[v.Colour])
		}
	}
	dotside := dots[0].Bounds().Max.X

	for _, v := range rpl.Messages {
		switch p := v.NetPacket.(type) {
		case packet.PkGameDroidInfo:
			if int(v.Player) > len(dots) {
				continue
			}
			dot := dots[v.Player]
			cx, cy := int((float64(p.CoordX)/128)*scale), int((float64(p.CoordY)/128)*scale)
			draw.Draw(img, image.Rect(cx-dotside, cy-dotside, cx+dotside, cy+dotside), dot, image.Point{}, draw.Over)
		}
	}

	ibuf := bytes.NewBuffer([]byte{})
	err := png.Encode(ibuf, img)
	return ibuf.Bytes(), err
}

func APIheadAnimatedReplayHeatmap(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gids := params["gid"]
	gid, err := strconv.Atoi(gids)
	if err != nil {
		return 400, nil
	}
	_, _, err = getReplayStuffs(gid)
	if err != nil {
		if err == errReplayNotFound {
			return 204, nil
		}
		return 500, err
	}
	w.Header().Set("Content-Type", "image/png")
	return 200, nil
}

func APIgetAnimatedReplayHeatmap(w http.ResponseWriter, r *http.Request) (int, any) {
	params := mux.Vars(r)
	gids := params["gid"]
	gid, err := strconv.Atoi(gids)
	if err != nil {
		return 400, nil
	}

	rpl, mapimg, err := getReplayStuffs(gid)
	if err != nil {
		if err == errReplayNotFound {
			return 204, nil
		}
		return 500, err
	}

	img, err := genReplayAnimatedHeatmap(r.Context(), *rpl, mapimg)
	if err != nil {
		return 500, err
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "image/png")
	w.Write(img)
	return -1, nil
}

func genReplayAnimatedHeatmap(ctx context.Context, rpl replay.Replay, mapimg image.Image) ([]byte, error) {
	const scale = 8
	const dotsize = 16
	const step = 10000
	const duration = step * 3

	const mapimgscale = scale

	log.Println("Scaling stuff...")
	smapimg := image.NewPaletted(image.Rectangle{Max: mapimg.Bounds().Max.Mul(mapimgscale)}, palette.WebSafe)
	draw.NearestNeighbor.Scale(smapimg, smapimg.Rect, mapimg, mapimg.Bounds(), draw.Src, nil)

	log.Println("Drawing dots...")
	dots := []draw.Image{}
	for _, v := range rpl.Settings.GameOptions.NetplayPlayers {
		dots = append(dots, mkDot(dotsize, playerColors[v.Colour]))
	}
	dotside := dots[0].Bounds().Max.X

	g := &gif.GIF{}

	log.Println("Rendering frames...")
	lastframeskip := 0
	for start := 0; start < rpl.End.GameTimeElapsed; start += step {
		frame := copyImage(smapimg)
		nowgt := 0
		collected := 0
		i := 0
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
	msgloop:
		for i = lastframeskip; i < len(rpl.Messages); i++ {
			switch p := rpl.Messages[i].NetPacket.(type) {
			case packet.PkGameDroidInfo:
				if nowgt < start {
					continue msgloop
				}
				dot := dots[rpl.Messages[i].Player]
				cx, cy := int((float64(p.CoordX)/128)*scale), int((float64(p.CoordY)/128)*scale)
				draw.Draw(frame, image.Rect(cx-dotside, cy-dotside, cx+dotside, cy+dotside), dot, image.Point{}, draw.Over)
				collected++
			case packet.PkGameGameTime:
				nowgt = int(p.GameTime)
				if nowgt < start+step {
					lastframeskip = i
				}
				if nowgt > start+duration {
					break msgloop
				}
			}
		}
		log.Printf("Frame %v collected %v skip %v gt %v i %v", start, collected, lastframeskip, nowgt, i)
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 5)
	}

	ibuf := bytes.NewBuffer([]byte{})
	err := gif.EncodeAll(ibuf, g)
	return ibuf.Bytes(), err
}

func copyImage(from *image.Paletted) *image.Paletted {
	ret := image.Paletted{
		Pix:     make([]uint8, len(from.Pix)),
		Stride:  from.Stride,
		Rect:    from.Rect,
		Palette: make(color.Palette, len(from.Palette)),
	}
	copy(ret.Pix, from.Pix)
	copy(ret.Palette, from.Palette)
	return &ret
}

func mkDot(size float64, c color.RGBA) draw.Image {
	i := image.NewRGBA(image.Rect(0, 0, int(size), int(size)))
	md := 0.5 * math.Sqrt(math.Pow(float64(size)/2.0, 2)+math.Pow((float64(size)/2.0), 2))
	for x := float64(0); x < size; x++ {
		for y := float64(0); y < size; y++ {
			d := math.Sqrt(math.Pow(x-size/2.0, 2) + math.Pow(y-size/2.0, 2))
			if d < md {
				rgbVal := uint8(200.0*d/md + 50.0)
				rgba := color.NRGBA{c.R, c.G, c.B, 255 - rgbVal}
				i.Set(int(x), int(y), rgba)
			}
		}
	}
	return i
}

var playerColors = []color.RGBA{
	{0, 255, 0, 255},
	{255, 255, 0, 255},
	{255, 255, 255, 255},
	{0, 0, 0, 255},
	{255, 0, 0, 255},
	{0, 0, 255, 255},
	{255, 0, 255, 255},
	{0, 255, 255, 255},
	{255, 255, 0, 255},
	{128, 0, 128, 255},
	{224, 224, 224, 255},
	{32, 32, 255, 255},
	{0, 160, 0, 255},
	{64, 0, 0, 255},
	{16, 0, 64, 255},
	{64, 96, 0, 255},
}
