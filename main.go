package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"image/color"
	"image/gif"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	logcolor "github.com/fatih/color"
	"github.com/fujiwara/logutils"
	"github.com/fujiwara/ridge"
	"github.com/golang/freetype/truetype"
	"github.com/mashiike/setddblock"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/math/fixed"
)

var (
	dynamoDBLockURL string
	s3Bucket        string
	s3ObjectPath    string
	client          *s3.Client
)

var filter = &logutils.LevelFilter{
	Levels: []logutils.LogLevel{"debug", "info", "warn", "error"},
	ModifierFuncs: []logutils.ModifierFunc{
		nil,
		nil,
		logutils.Color(logcolor.FgYellow),
		logutils.Color(logcolor.FgRed, logcolor.BgBlack),
	},
	Writer: os.Stderr,
}

func main() {
	minLevel := "info"
	if os.Getenv("DEBUG") != "" {
		minLevel = "debug"
	}
	filter.MinLevel = logutils.LogLevel(minLevel)

	log.SetOutput(filter)
	dynamoDBLockURL = os.Getenv("DDB_LOCK_URL")
	if dynamoDBLockURL == "" {
		log.Println("[error] DDB_LOCK_URL is required")
		os.Exit(1)
	}
	s3Bucket = os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		log.Println("[error] S3_BUCKET is required")
		os.Exit(1)
	}
	s3ObjectPath = os.Getenv("S3_OBJECT_PATH")
	if s3ObjectPath == "" {
		log.Println("[error] S3_OBJECT_PATH is required")
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Println("config load failed")
		os.Exit(1)
	}
	client = s3.NewFromConfig(cfg)

	var mux = http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/counter.gif", handleCounterImage)
	mux.HandleFunc("/healthcheck", handleHealthcheck)
	ridge.Run(":8080", "/", mux)
}

func handleHealthcheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "200 OK")
}

func handleCounterImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	counter, err := getCounter(r.Context())
	if err != nil {
		log.Printf("[error] get counter %#v\n", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	img, err := generateCounterImage(counter)
	if err != nil {
		log.Printf("[error] generate image %#v\n", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	var imgBuf bytes.Buffer
	err = gif.Encode(&imgBuf, img, &gif.Options{
		NumColors: 256,
	})
	if err != nil {
		log.Printf("[error] image encode %#v\n", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/gif")
	io.Copy(w, &imgBuf)
}

type Counter struct {
	Visit      int64     `json:"visit,omitempty"`
	LastAccess time.Time `json:"last_access,omitempty"`
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "text/html")
			log.Println("[info] head access")
			return
		}
		errResponseWriter(w, r, "", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("[info] access %s", r.Header.Get("User-Agent"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger := log.New(filter, "", log.LstdFlags)
	l, err := setddblock.New(
		dynamoDBLockURL,
		setddblock.WithContext(ctx),
		setddblock.WithDelay(true),
		setddblock.WithLeaseDuration(100*time.Millisecond),
		setddblock.WithLogger(logger),
	)
	if err != nil {
		log.Println("[error] can not init stddblock", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	lockGranted, err := l.LockWithErr(ctx)
	if err != nil {
		log.Println("[error] get lock stddblock", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	if !lockGranted {
		log.Println("[error] get lock stddblock", err)
		errResponseWriter(w, r, "lcok was not granted", http.StatusGatewayTimeout)
		return
	}
	defer l.Unlock()
	counter, err := getCounter(ctx)
	if err != nil {
		log.Printf("[error] get counter %#v\n", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	counter.LastAccess = time.Now()
	counter.Visit++
	log.Printf("[info] now visit %d (last access %s)", counter.Visit, counter.LastAccess)
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(counter); err != nil {
		log.Printf("[error] json encode error %#v\n", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s3Bucket,
		Key:    &s3ObjectPath,
		Body:   &buf,
	})
	if err != nil {
		log.Printf("[error] put s3 object %#v\n", err)
		errResponseWriter(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<html><head><title>modern-access-counter</title></head><body>%d<br><img src="/counter.gif"/><br></body><html>`, counter.Visit)
}

func errResponseWriter(w http.ResponseWriter, r *http.Request, msg string, code int) {
	w.Header().Set("Content-Type", "text/plain")
	if msg == "" {
		http.Error(w, http.StatusText(code), code)
	} else {
		http.Error(w, msg, code)
	}
}

func getCounter(ctx context.Context) (Counter, error) {
	getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s3Bucket,
		Key:    &s3ObjectPath,
	})
	var counter Counter
	if err != nil {
		var nsk *types.NoSuchKey
		if !errors.As(err, &nsk) {
			err = nil
		}
		return counter, err
	}

	defer getOutput.Body.Close()
	decoder := json.NewDecoder(getOutput.Body)
	if err := decoder.Decode(&counter); err != nil {
		return counter, err
	}
	return counter, nil
}

//from https://qiita.com/tng527/items/7af65659f7666a122da2
func generateCounterImage(counter Counter) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, 240, 60))
	tt, err := truetype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	fontsize := float64(img.Rect.Dx()) * 0.25 * 0.8 / 1.333

	d := &font.Drawer{
		Dst: img,
		Src: image.NewUniform(color.White),
		Face: truetype.NewFace(
			tt, &truetype.Options{
				Size: fontsize,
			},
		),
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(((float64(img.Rect.Dx()) / 2) - fontsize*2/1.333) * 64),
			Y: fixed.Int26_6((img.Rect.Dy() - 20) * 64),
		},
	}
	d.DrawString(fmt.Sprintf("%04d", counter.Visit))
	return img, nil
}
