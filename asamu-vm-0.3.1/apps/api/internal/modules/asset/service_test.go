package asset

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestSlotBindingsAreNotParsedAsDatabaseRelation(t *testing.T) {
	parsed, err := schema.Parse(&Slot{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if field := parsed.LookUpField("Bindings"); field != nil && field.DBName != "" {
		t.Fatalf("Bindings unexpectedly mapped to database column %q", field.DBName)
	}
}

func TestInspectTeamAvatarAcceptsRasterImageAndRejectsSVG(t *testing.T) {
	var data bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, 32, 32))
	source.Set(0, 0, color.RGBA{R: 20, G: 80, B: 220, A: 255})
	if err := png.Encode(&data, source); err != nil {
		t.Fatal(err)
	}
	metadata, err := inspectTeamAvatar(Upload{Name: "avatar.png", Data: data.Bytes()})
	if err != nil {
		t.Fatalf("valid team avatar rejected: %v", err)
	}
	if metadata.MIME != "image/png" || metadata.Width != 32 || metadata.Height != 32 {
		t.Fatalf("unexpected team avatar metadata: %#v", metadata)
	}
	if _, err := inspectTeamAvatar(Upload{Name: "avatar.svg", Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)}); err == nil {
		t.Fatal("SVG team avatar should be rejected")
	}
}
