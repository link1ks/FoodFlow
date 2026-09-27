// modelcheck makes three small, billable requests using synthetic test data only.
package main

import (
	"bytes"
	"context"
	"fmt"
	"foodflow/internal/agent"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"image"
	"image/color"
	"image/png"
	"os"
	"time"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Println("Invalid .env syntax")
		os.Exit(1)
	}
	m := agent.OpenAIModel{Endpoint: os.Getenv("MODEL_ENDPOINT"), Name: os.Getenv("MODEL_NAME"), Key: os.Getenv("MODEL_API_KEY"), MaxTokens: 600}
	if m.Endpoint == "" || m.Name == "" || m.Key == "" {
		fmt.Println("Text model configuration incomplete")
		os.Exit(1)
	}
	failed := false
	check := func(name string, f func(context.Context) error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := f(ctx); err != nil {
			fmt.Printf("%s: FAIL (%v)\n", name, err)
			failed = true
		} else {
			fmt.Println(name + ": PASS")
		}
	}
	check("menu", func(ctx context.Context) error {
		r, e := m.Generate(ctx, []*schema.Message{schema.SystemMessage("Return JSON only: {\"recipe_id\":\"test-salad\"}. Select only this allowed recipe."), schema.UserMessage("Synthetic test: lettuce salad for two people.")})
		if e != nil {
			return e
		}
		id, e := agent.ParseRecipeID(r.Content)
		if e != nil {
			return e
		}
		if id != "test-salad" {
			return fmt.Errorf("unknown recipe")
		}
		return nil
	})
	check("nutrition", func(ctx context.Context) error {
		r, e := m.Generate(ctx, []*schema.Message{schema.SystemMessage("Return JSON with summary (Chinese string), tips (1-3 Chinese strings), recipe_ids (array containing only test-salad). Give general cooking advice, no invented nutrient values."), schema.UserMessage("Synthetic test inventory: 300 g lettuce. Allowed recipe test-salad: lettuce salad. Two people.")})
		if e != nil {
			return e
		}
		_, e = agent.ParseAdvice(r.Content, map[string]bool{"test-salad": true})
		return e
	})
	check("vision", func(ctx context.Context) error {
		// Simple synthetic tomato illustration; no user photo leaves this machine.
		img := image.NewRGBA(image.Rect(0, 0, 256, 256))
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				c := color.RGBA{255, 255, 255, 255}
				if (x-128)*(x-128)+(y-145)*(y-145) < 85*85 {
					c = color.RGBA{230, 45, 40, 255}
				}
				if x > 118 && x < 138 && y > 30 && y < 75 {
					c = color.RGBA{30, 140, 45, 255}
				}
				img.SetRGBA(x, y, c)
			}
		}
		var b bytes.Buffer
		if e := png.Encode(&b, img); e != nil {
			return e
		}
		_, e := agent.VisionFromEnv().Recognize(ctx, b.Bytes(), "image/png")
		return e
	})
	if failed {
		os.Exit(1)
	}
}
