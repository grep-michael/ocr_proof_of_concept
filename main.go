package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/sashabaranov/go-openai"
	"gocv.io/x/gocv"
)

//const OCR_MODEL = "deepseek-ai/DeepSeek-OCR"
//const PROMPT = "Free OCR."

const OCR_MODEL = "Qwen/Qwen2.5-VL-7B-Instruct"
const PROMPT = "Read all the text in the image."

type CameraWidget struct {
	widget.BaseWidget
	frozen      bool
	lowThresh   binding.Float
	highThresh  binding.Float
	canvasImage *canvas.Image
	webcam      *gocv.VideoCapture
	stopCh      chan struct{}
}

func NewCameraWidget(deviceID int) (*CameraWidget, error) {
	webcam, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		return nil, err
	}

	img := canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 640, 480)))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(640, 480))

	cw := &CameraWidget{
		canvasImage: img,
		webcam:      webcam,
		lowThresh:   binding.NewFloat(),
		highThresh:  binding.NewFloat(),
		stopCh:      make(chan struct{}),
	}
	cw.lowThresh.Set(50)
	cw.highThresh.Set(150)
	cw.ExtendBaseWidget(cw)
	return cw, nil
}

func (cw *CameraWidget) CreateRenderer() fyne.WidgetRenderer {

	vbox := container.NewVBox(
		container.NewStack(cw.canvasImage),
	)
	return widget.NewSimpleRenderer(
		vbox,
	)
}

func (cw *CameraWidget) Start() {
	go func() {
		frame := gocv.NewMat()
		defer frame.Close()
		for {
			select {
			case <-cw.stopCh:
				return
			default:

				if cw.frozen {
					continue
				}

				if ok := cw.webcam.Read(&frame); !ok || frame.Empty() {
					continue
				}

				gocv.CvtColor(frame, &frame, gocv.ColorBGRToRGBA)
				gocv.CvtColor(frame, &frame, gocv.ColorRGBAToGray)
				//gocv.GaussianBlur(frame, &frame, image.Pt(3, 3), 0, 0, gocv.BorderConstant)

				//t1, err := cw.lowThresh.Get()
				//t2, err := cw.highThresh.Get()

				//err = gocv.Canny(frame, &frame, float32(t1), float32(t2))
				//if err != nil {
				//	log.Println(err)
				//}

				img, _ := frame.ToImage()
				fyne.Do(func() {
					cw.canvasImage.Image = img
					canvas.Refresh(cw.canvasImage)
				})
			}
		}
	}()
}

func (cw *CameraWidget) Stop() {
	close(cw.stopCh)
	cw.webcam.Close()
}

var regexs = []string{
	`INNO\s*LIGHT.*(\w-.{3,7}-.{1,4})`,
	`P\/N:\s*(.*)\s+`,
	`PN:\s*(.*)\s+`,
	`(?m)^(\w{0,5}-\w{0,5}-\w{0,5}-\w{0,5})`,
	`(\w{1,3}-\w{0,7}-\w{0,5})`,
}

//var model_regex = regexp.MustCompile(`INNO\s*LIGHT.*(\w-.{3,7}-.{1,4})`)

func findModel(response string) (string, error) {
	response = strings.ToUpper(response)
	for _, regex := range regexs {
		r := regexp.MustCompile(regex)
		matches := r.FindStringSubmatch(response)
		if matches == nil {
			continue
		}
		filename := strings.TrimSpace(matches[1]) + ".txt"
		count := 0
		if data, err := os.ReadFile(filename); err == nil {
			count, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		count++
		if err := os.WriteFile(filename, []byte(strconv.Itoa(count)), 0644); err != nil {
			//log.Printf("failed to write %s: %+v", filename, err)
			return "", fmt.Errorf("Couldnt write file")
		}
		//fmt.Printf("matched: %q — count now %d (file: %s)\n", matches[1], count, filename)
		return matches[1], nil

	}
	return "", fmt.Errorf("No Matches")
}
func UploadButton(cam *CameraWidget, window fyne.Window) fyne.CanvasObject {
	label := widget.NewLabel("")
	btn := widget.NewButton("OCR", func() {
		img := cam.canvasImage.Image
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			log.Println(err)
			return
		}
		b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
		config := openai.DefaultConfig("EMPTY")
		config.BaseURL = "http://10.10.0.6:8000/v1"
		client := openai.NewClientWithConfig(
			config,
		)
		resp, err := client.CreateChatCompletion(context.Background(),
			openai.ChatCompletionRequest{
				Model:     OCR_MODEL,
				MaxTokens: 2048,
				Messages: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleUser,
						MultiContent: []openai.ChatMessagePart{
							{
								Type: openai.ChatMessagePartTypeImageURL,
								ImageURL: &openai.ChatMessageImageURL{
									URL: "data:image/jpeg;base64," + b64,
								},
							},
							{
								Type: openai.ChatMessagePartTypeText,
								Text: PROMPT,
							},
						},
					},
				},
			},
		)
		if err != nil {
			log.Printf("Client error: %+v\n", err)
			return
		}
		log.Printf("\n\t%s\n\n", resp.Choices[0].Message.Content)
		s, err := findModel(resp.Choices[0].Message.Content)
		if err != nil {
			dialog.ShowConfirm("Error", err.Error(), func(bool) {
			}, window)
		} else {
			fyne.Do(func() {
				label.SetText(s)
			})
		}
	})

	return container.NewVBox(label, btn, cam)
}

func main() {
	a := app.New()
	w := a.NewWindow("Camera Test")

	cam, err := NewCameraWidget(0)
	if err != nil {
		log.Fatalf("failed to open camera: %v", err)
	}

	cam.Start()

	w.SetContent(UploadButton(cam, w))
	w.Resize(fyne.NewSize(640, 480))
	w.SetOnClosed(func() {
		cam.Stop()
	})
	w.ShowAndRun()
}
