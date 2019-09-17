package main

import (
	"errors"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/muesli/sticker"
	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
)

const (
	maxSize         = 4000
	defaultSize     = 512
	backgroundImage = "/tmp/bg.jpg"
	ttf             = "/usr/share/fonts/TTF/Roboto-Bold.ttf"
	defaultFg       = "#969696"
	defaultBg       = "#cccccc"
	bind            = "127.0.0.1:3000"
)

// convert color hex-code to rgb
func hexToRGB(h string) (uint8, uint8, uint8, error) {
	rgb, err := strconv.ParseUint(string(h), 16, 32)
	if err == nil {
		return uint8(rgb >> 16), uint8((rgb >> 8) & 0xFF), uint8(rgb & 0xFF), nil
	}
	return 0, 0, 0, err
}

// normalize hex color
func normalizeHex(h string) string {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 3 && len(h) != 6 {
		return ""
	}
	if len(h) == 3 {
		h = h[:1] + h[:1] + h[1:2] + h[1:2] + h[2:] + h[2:]
	}
	return h
}

func paramToColor(param, defaultValue string) (color.RGBA, error) {
	if len(param) == 0 {
		param = defaultValue
	}

	hexColor := normalizeHex(param)
	if len(hexColor) == 0 {
		return color.RGBA{}, errors.New("bad hex color format")
	}

	R, G, B, err := hexToRGB(hexColor)
	if err != nil {
		return color.RGBA{}, err
	}

	return color.RGBA{R, G, B, 255}, nil
}

func serveImage(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	r.ParseForm()

	text := params["text"]
	width, _ := strconv.ParseInt(r.FormValue("width"), 10, 32)
	height, _ := strconv.ParseInt(r.FormValue("height"), 10, 32)
	if width == 0 && height == 0 {
		width = defaultSize
	}

	if width > maxSize || height > maxSize {
		http.Error(w, "Image too large", http.StatusRequestEntityTooLarge)
		log.WithFields(log.Fields{
			"width":  width,
			"height": height,
		}).Warn("requested image too large")
		return
	}

	foregroundValue := r.FormValue("fg")
	backgroundValue := r.FormValue("bg")
	fg, err := paramToColor(foregroundValue, defaultFg)
	if err != nil {
		http.Error(w, "Bad value for foreground color", http.StatusBadRequest)
		log.WithField("color", foregroundValue).Error(err)
		return
	}
	bg, err := paramToColor(backgroundValue, defaultBg)
	if err != nil {
		http.Error(w, "Bad value for background color", http.StatusBadRequest)
		log.WithField("color", backgroundValue).Error(err)
		return
	}

	var bgimg image.Image
	bgfile, err := os.Open(backgroundImage)
	if err != nil {
		log.Error(err)
	} else {
		defer bgfile.Close()

		bgimg, _, err = image.Decode(bgfile)
		if err != nil {
			log.Error(err)
			return
		}
	}

	ph, err := sticker.NewImageGenerator(sticker.Options{
		TTFPath:         ttf,
		MarginRatio:     -1,
		Foreground:      fg,
		Background:      bg,
		BackgroundImage: bgimg,
	})
	if err != nil {
		http.Error(w, "Could not create generator", http.StatusBadRequest)
		log.WithField("ttf", ttf).Error(err)
		return
	}

	img, err := ph.NewPlaceholder(text, int(width), int(height))
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)

	imgName := params["width"]
	if w, ok := params["height"]; ok {
		imgName += "x" + w
	}
	imgName += ".png"

	log.WithFields(log.Fields{
		"width":      width,
		"height":     height,
		"foreground": fg,
		"background": bg,
		"text":       text,
	}).Infof("Served image \"%s\"", imgName)

}

func main() {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/{text}.png", serveImage).Methods("GET")

	http.Handle("/", rtr)

	log.Infof("Starting quaint on %s", bind)
	log.Fatal(http.ListenAndServe(bind, nil))
}
