// Copyright (C) 2026  Davi Silva Valério Diniz <davi.diniz101@gmail.com>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <https://www.gnu.org/licenses/>.
package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const SCROLLING_TITLE_SPEED = 100
const SCROLLING_TEXT_INITIAL_WAIT = 3

var DarkRed = rl.Color{117, 0, 44, 255}
var DarkDarkRed = rl.Color{117 - 20, 0, 44 - 20, 255}
var DarkYellow = rl.Color{153, 149, 0, 255}
var COLORS = []rl.Color{
	rl.LightGray, rl.Gray, rl.DarkGray, rl.Yellow, rl.Gold,
	rl.Orange, rl.Pink, rl.Red, rl.Maroon, rl.Green, rl.Lime,
	rl.DarkGreen, rl.SkyBlue, rl.Blue, rl.DarkBlue, rl.Purple,
	rl.Violet, rl.DarkPurple, rl.Beige, rl.Brown, rl.DarkBrown,
	rl.White, rl.Black, rl.Blank, rl.Magenta, rl.RayWhite, DarkRed,
	DarkYellow,
}

var Alloc uint64
var TotalAlloc uint64
var Sys uint64
var NumGC uint32

func MonitorMemory() {
	for {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		Alloc = m.Alloc / 1024 / 1024
		TotalAlloc = m.TotalAlloc / 1024 / 1024
		Sys = m.Sys / 1024 / 1024
		NumGC = m.NumGC
		time.Sleep(5 * time.Second)
	}
}

func DrawDebugInfo(font Font, bounds *rl.Rectangle, t BoxTextTheme) {
	const padding = 10
	statusBarText := fmt.Sprintf("FPS: %d FrameTime:%.2fms Alloc: %vMiB TotalAlloc: %vMiB Sys: %vMiB NumGC: %v", rl.GetFPS(), rl.GetFrameTime()*1000, Alloc, TotalAlloc, Sys, NumGC)
	//statusBarText += fmt.Sprintf(" U.LayerIndex: %d U.Layers: %d", uiInput.User.LayerIndex, uiInput.User.Layers)
	text, lines := WrapMonospaceSlow(
		[]rune(statusBarText),
		font.CharacterWidth, font.Spacing, bounds.Width-padding*2,
	)
	fpsInfoArea := CutTop(bounds, font.Padding*float32(lines))
	rl.DrawRectangleRec(fpsInfoArea, t.Bg)
	rl.DrawRectangleLinesEx(fpsInfoArea, t.Thickness, t.Border)
	textPos := rl.Vector2{
		fpsInfoArea.X + padding,
		fpsInfoArea.Y + (fpsInfoArea.Height-(font.Size*float32(lines)))/2,
	}
	DrawText(font, textPos, string(text), t.Text)
}

func GetSystemFont() (string, error) {
	cmd := exec.Command("fc-match", "--format=%{file}\n", "monospace")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running fc-match: %s", err)
	}
	fontPath := strings.TrimSpace(string(out))
	return fontPath, nil
}

func GenerateExtendedChars(amount int64) []rune {
	extendedChars := make([]rune, amount)
	for i := 0; i < len(extendedChars); i++ {
		extendedChars[i] = rune(i)
	}
	return extendedChars
}

type Font struct {
	Font              rl.Font
	Resolution        int32
	Size              float32
	Spacing           float32
	LineSpacing       float32
	WhiteSpaceWidth   float32
	Padding           float32
	CharacterWidth    float32
	IsMonospace       bool
	MeasuredTextCache map[string]rl.Vector2
}

func NewFont(fontPath string, font_size float32, codepointSize int64) Font {
	font_resolution := int32(font_size * 3)
	font_spacing := font_size * 0.05
	if font_size <= 14 {
		fmt.Printf("raylib does not render very small ttf fonts - yours is %f, bitmap fonts are better in this case\n", font_size)
		font_spacing = 0
	}
	line_spacing := font_size * 0.25
	font := NewFontEx(fontPath, font_resolution, font_size, font_spacing, line_spacing, codepointSize)
	return font
}
func NewFontEx(fontPath string, font_resolution int32, font_size, font_spacing, line_spacing float32, codepointSize int64) Font {
	rlFont := rl.LoadFontEx(fontPath, font_resolution, GenerateExtendedChars(codepointSize))
	font := Font{rlFont, font_resolution, font_size, font_spacing, line_spacing, 0, 0, 0, false, make(map[string]rl.Vector2, 0)}
	font.WhiteSpaceWidth = rl.MeasureTextEx(font.Font, " ", font.Size, font.Spacing).X
	font.Padding = font.Size * 1.5
	font.CharacterWidth = MeasureText(font, "A").X
	rl.SetTextureFilter(font.Font.Texture, rl.FilterBilinear)
	return font
}
func NewFontInMemory(fontBytes []byte, font_size float32, codepointSize int64) Font {
	font_resolution := int32(font_size * 3)
	font_spacing := font_size * 0.05
	if font_size <= 14 {
		fmt.Printf("raylib does not render very small ttf fonts - yours is %f, bitmap fonts are better in this case\n", font_size)
		font_spacing = 0
	}
	line_spacing := font_size * 0.25
	rlFont := rl.LoadFontFromMemory(".ttf", fontBytes, font_resolution, GenerateExtendedChars(codepointSize))
	font := Font{rlFont, font_resolution, font_size, font_spacing, line_spacing, 0, 0, 0, false, make(map[string]rl.Vector2, 0)}
	font.WhiteSpaceWidth = rl.MeasureTextEx(font.Font, " ", font.Size, font.Spacing).X
	font.Padding = font.Size * 1.5
	font.CharacterWidth = MeasureText(font, "A").X
	rl.SetTextureFilter(font.Font.Texture, rl.FilterPoint)
	return font
}
func (f Font) Em(x float32) float32 {
	return f.Size * x
}
func (f Font) LineSize() float32 {
	return f.Size + f.LineSpacing
}

// goes char by char, looks for "\n"
func WrapMonospaceSlow(
	str []rune,
	charWidth, spacing, maxWidth float32,
) (_result []rune, _lines int) {
	maxChars := int(Ceil(maxWidth / (charWidth + spacing)))
	_result = make([]rune, 0)
	count := 0
	_lines = 0
	for _, r := range str {
		_result = append(_result, r)
		if r == '\n' {
			_lines++
			count = 0
		} else {
			count++
		}
		if count >= maxChars {
			_result = append(_result, '\n')
			_lines++
			count = 0
		}
	}
	return _result, _lines + 1
}
func WrapMonospace2(
	str []rune,
	charWidth, spacing, maxWidth float32,
) (_result []string) {
	r, lines := WrapMonospaceSlow(str, charWidth, spacing, maxWidth)
	txt := string(r)
	_result = make([]string, lines)
	for i, s := range strings.Split(txt, "\n") {
		_result[i] = s
	}
	return _result
}

func WordWrapMonospaceWhitespace(
	input []rune,
	charSize, spacing, maxWidth float32,
) (_result []rune, _lines int) {
	_result = make([]rune, len(input))
	j := 0
	currentWidth := float32(0)
	lineStart := 0
	lastWhitespace := 0
	for _, r := range input {
		newWidth := currentWidth + charSize + spacing
		if newWidth < maxWidth {
			currentWidth = newWidth
			if unicode.IsSpace(r) {
				lastWhitespace = j
			}
			_result[j] = r
		} else {
			currentWidth = 0
			if unicode.IsSpace(r) {
				_result[j] = '\n'
				lineStart = j
			} else {
				if lastWhitespace > lineStart {
					_result[lastWhitespace] = '\n'
					lineStart = lastWhitespace
				} else {
					_result = slices.Insert(_result, j-2, '\n')
					j++
					lineStart = j
				}
				_result[j] = r
			}
		}
		j++
	}
	for _, r := range _result {
		if r == '\n' {
			_lines++
		}
	}
	return _result, _lines
}

func DrawText(font Font, pos rl.Vector2, text string, color rl.Color) {
	rl.DrawTextEx(font.Font, text, pos, font.Size, font.Spacing, color)
}
func DrawTextXY(font Font, x, y float32, text string, color rl.Color) {
	rl.DrawTextEx(font.Font, text, rl.Vector2{x, y}, font.Size, font.Spacing, color)
}

func MeasureText(font Font, text string) rl.Vector2 {
	if font.IsMonospace {
		return rl.Vector2{float32(utf8.RuneCountInString(text)) * (font.CharacterWidth + font.Spacing), font.Size}
	} else {
		dim, ok := font.MeasuredTextCache[text]
		if ok {
			return dim
		} else {
			dim = rl.MeasureTextEx(font.Font, text, font.Size, font.Spacing)
			font.MeasuredTextCache[text] = dim
			return dim
		}
	}
}
func DrawBox(bounds rl.Rectangle, thickness float32, background, border rl.Color) {
	rl.DrawRectangleRec(bounds, background)
	rl.DrawRectangleLinesEx(bounds, thickness, border)
}
func DrawBoxEx(bounds rl.Rectangle, theme BoxTheme) {
	rl.DrawRectangleRec(bounds, theme.Bg)
	rl.DrawRectangleLinesEx(bounds, theme.Thickness, theme.Border)
}
func DrawBoxEx2(bounds rl.Rectangle, theme BoxTextTheme) {
	rl.DrawRectangleRec(bounds, theme.Bg)
	rl.DrawRectangleLinesEx(bounds, theme.Thickness, theme.Border)
}

func CutRight(r *rl.Rectangle, width float32) rl.Rectangle {
	w := min(r.Width, width)
	right := rl.Rectangle{
		r.X + r.Width - w, r.Y,
		w, r.Height,
	}
	r.Width -= w
	return right
}
func CutLeft(r *rl.Rectangle, width float32) rl.Rectangle {
	w := min(r.Width, width)
	left := rl.Rectangle{
		r.X, r.Y,
		w, r.Height,
	}
	r.Width -= w
	r.X += w
	return left
}
func CutTop(r *rl.Rectangle, height float32) rl.Rectangle {
	h := min(r.Height, height)
	top := rl.Rectangle{
		r.X, r.Y,
		r.Width, h,
	}
	r.Height -= h
	r.Y += h
	return top
}
func CutBottom(r *rl.Rectangle, height float32) rl.Rectangle {
	h := min(r.Height, height)
	down := rl.Rectangle{
		r.X, r.Y + r.Height - h,
		r.Width, h,
	}
	r.Height -= h
	return down
}
func CutVertical(r rl.Rectangle, width float32) (_left, _right rl.Rectangle) {
	left := CutLeft(&r, width)
	return left, r
}
func CutHorizontal(r rl.Rectangle, width float32) (_top, _down rl.Rectangle) {
	top := CutTop(&r, width)
	return top, r
}
func Pad(r rl.Rectangle, pad float32) rl.Rectangle {
	return rl.Rectangle{
		r.X + pad,
		r.Y + pad,
		r.Width - pad*2,
		r.Height - pad*2,
	}
}
func PadEx(r rl.Rectangle, padWidth, padHeight float32) rl.Rectangle {
	return rl.Rectangle{
		r.X + padWidth,
		r.Y + padHeight,
		r.Width - padWidth*2,
		r.Height - padHeight*2,
	}
}
func Margin(r rl.Rectangle, margin float32) rl.Rectangle {
	return MarginEx(r, margin, margin)
}
func MarginEx(r rl.Rectangle, marginWidth, marginHeight float32) rl.Rectangle {
	return rl.Rectangle{
		r.X - marginWidth,
		r.Y - marginHeight,
		r.Width + marginWidth,
		r.Height + marginHeight,
	}
}
func BeginScissorMode(rec rl.Rectangle) {
	rl.BeginScissorMode(int32(rec.X), int32(rec.Y), int32(rec.Width), int32(rec.Height))
}

type TextJustify int

const (
	TEXT_CENTER TextJustify = iota
	TEXT_RIGHT
	TEXT_LEFT
)

type BoxTextTheme struct {
	Thickness float32
	Text      rl.Color
	Border    rl.Color
	Bg        rl.Color
}
type BoxTheme struct {
	Thickness float32
	Border    rl.Color
	Bg        rl.Color
}

func DrawBoundedText(
	font Font,
	text string,
	bounds rl.Rectangle,
	textJustify TextJustify,
	textColor, bg rl.Color,
) {
	lines := strings.Split(text, "\n")
	height := font.Size
	if len(lines) > 1 {
		height = float32(len(lines)) * (font.LineSize())
	}
	pos := rl.Vector2{}
	pos.Y = bounds.Y + (bounds.Height-height)/2
	for _, l := range lines {
		dim := MeasureText(font, l)
		switch textJustify {
		case TEXT_CENTER:
			pos.X = bounds.X + (bounds.Width-dim.X)/2
		case TEXT_RIGHT:
			pos.X = bounds.X + bounds.Width - dim.X
		case TEXT_LEFT:
			pos.X = bounds.X + font.Spacing
		}
		rl.DrawRectangleRec(rl.Rectangle{pos.X, pos.Y, dim.X, dim.Y}, bg)
		DrawText(font, pos, l, textColor)
		pos.Y += font.LineSize()
	}
}
func DrawTextBox(
	font Font,
	text string,
	bounds rl.Rectangle,
	textJustify TextJustify,
	theme BoxTextTheme,
) {
	dim := MeasureText(font, text)
	DrawBox(bounds, theme.Thickness, theme.Bg, theme.Border)
	pos := rl.Vector2{}
	switch textJustify {
	case TEXT_CENTER:
		pos.X = bounds.X + (bounds.Width-dim.X)/2
		pos.Y = bounds.Y + (bounds.Height-dim.Y)/2
	case TEXT_RIGHT:
		pos.X = bounds.X + bounds.Width - dim.X
		pos.Y = bounds.Y + (bounds.Height-dim.Y)/2
	case TEXT_LEFT:
		pos.X = bounds.X + (font.Spacing * 2)
		pos.Y = bounds.Y + (bounds.Height-dim.Y)/2
	}
	DrawText(font, pos, text, theme.Text)
}
func TextBox2(
	font Font,
	text string,
	width float32,
	height float32,
	bounds rl.Rectangle,
	textJustify TextJustify,
	textColor, border, background rl.Color, thickness float32,
) {
	DrawBox(bounds, thickness, background, border)
	pos := rl.Vector2{}
	switch textJustify {
	case TEXT_CENTER:
		pos.X = bounds.X + (bounds.Width-width)/2
		pos.Y = bounds.Y + (bounds.Height-height)/2
	case TEXT_RIGHT:
		pos.X = bounds.X + bounds.Width - width
		pos.Y = bounds.Y + (bounds.Height-height)/2
	case TEXT_LEFT:
		pos.X = bounds.X + (font.Spacing * 2)
		pos.Y = bounds.Y + (bounds.Height-height)/2
	}
	DrawText(font, pos, text, textColor)
}

type ScrollFilter[T comparable] struct {
	SLE         *SingleLineEditor
	IsFiltering bool
	Items       []T
	Cursor      int
}

func NewScrollFilter[T comparable]() *ScrollFilter[T] {
	return &ScrollFilter[T]{
		SLE: NewSingleLineEditor(),
	}
}

// use the constructor functions because there's a bunch of fields that need default values
// and go doesn't have default values
type Scroll[T comparable] struct {
	Cursor              int
	OffsetY             int
	PaddingX            float32
	GapY                float32
	TotalItems          int
	ItemHeight          float32
	MaxDisplayableItems int

	//specific for tree scroll
	TreeTabSize float32

	//specific for filter
	FilterSLE           *SingleLineEditor
	PreviousFilterValue string
	FilterInputActive   bool
	FilterItems         []T
	FilterIndexes       []int
	FilterCursor        int

	//mouse input only
	Hitboxes []rl.Rectangle

	DirtyRectangle
}

func NewScroll[T comparable](itemHeight float32) Scroll[T] {
	return Scroll[T]{
		ItemHeight: itemHeight,
	}
}

func NewTreeScroll[T comparable](itemHeight, tabSize float32) Scroll[T] {
	return Scroll[T]{
		ItemHeight:  itemHeight,
		TreeTabSize: tabSize,
	}
}

func GetPuck(
	scrollBar rl.Rectangle,
	offsetY int,
	maxDisplayableRows int,
	totalRows int,
	maxColumns int,
	minSize float32,
) rl.Rectangle {
	if totalRows == 0 || maxColumns == 0 {
		return scrollBar
	}
	maxOffset := totalRows - maxDisplayableRows
	puckHeight := Max((float32(maxDisplayableRows)/float32(totalRows))*scrollBar.Height, minSize)
	var puckOffset float32
	if maxOffset > 0 {
		puckOffset = float32(offsetY) / float32(maxOffset) * (scrollBar.Height - puckHeight)
	}
	puckOffset = Max(0, Min(puckOffset, scrollBar.Height-puckHeight))
	return rl.Rectangle{scrollBar.X, scrollBar.Y + puckOffset, scrollBar.Width, puckHeight}
}

func DrawScrollFilter[T comparable](
	font Font,
	bounds *rl.Rectangle,
	filter *ScrollFilter[T],
	theme ScrollTheme,
	dt float32,
) {
	if filter == nil {
		return
	}
	showFilter := filter.IsFiltering || (filter.SLE != nil && len(filter.SLE.Text) > 0)
	if showFilter {
		const label_text = "Filter: "
		filterBounds := CutTop(bounds, font.Size)
		labelBounds := CutLeft(&filterBounds, MeasureText(font, label_text).X)
		DrawTextBox(font, label_text, labelBounds, TEXT_LEFT, theme.Filter)
		DrawBox(filterBounds, theme.Filter.Thickness, theme.Filter.Bg, theme.Filter.Border)
		filterLabelPos := rl.Vector2{filterBounds.X + font.WhiteSpaceWidth, filterBounds.Y}
		BeginScissorMode(filterBounds)
		DrawSLEEx(font, filter.SLE, filterLabelPos, theme.SLE, filter.IsFiltering, dt)
		rl.EndScissorMode()
	}
}
func IsFilterActive[T comparable](s Scroll[T]) bool {
	return s.FilterSLE != nil && (s.FilterInputActive || len(s.FilterSLE.Text) > 0)
}

type ScrollTheme struct {
	BoxTheme
	Puck               rl.Color
	PuckThickness      float32
	MinPuckSize        float32
	CursorPuck         rl.Color
	SelectedCursorPuck rl.Color
	MinCursorPuckSize  float32
	Filter             BoxTextTheme
	SLE                SingleLineEditorTheme
}

func NewDefaultScrollTheme() ScrollTheme {
	return ScrollTheme{
		BoxTheme:           BoxTheme{1, rl.White, rl.Black},
		Puck:               rl.Red,
		PuckThickness:      8,
		MinPuckSize:        10,
		CursorPuck:         rl.Blue,
		SelectedCursorPuck: rl.Yellow,
		MinCursorPuckSize:  5,
		Filter:             BoxTextTheme{1, rl.Yellow, rl.Yellow, rl.Black},
		SLE:                SingleLineEditorTheme{rl.Yellow, rl.White, rl.Blue},
	}
}

func DrawScroll[T comparable](
	font Font,
	bounds rl.Rectangle,
	s *Scroll[T],
	allItems []T,
	selectedIndexes map[int]struct{},
	dt float32,
	drawItem func(itemBounds rl.Rectangle, index int, item T, selected, highlighted bool),
) {
	defaultTheme := NewDefaultScrollTheme()
	DrawScrollEx(font, bounds, s, allItems, selectedIndexes, defaultTheme, dt, drawItem)
}

func DrawScrollEx[T comparable](
	font Font,
	bounds rl.Rectangle,
	s *Scroll[T],
	allItems []T,
	selectedIndexes map[int]struct{},
	theme ScrollTheme,
	dt float32,
	drawItem func(itemBounds rl.Rectangle, index int, item T, selected, highlighted bool),
) {
	const (
		label_text = "Filter: "
	)

	if rerender, textureBounds := StartDirtyRectangle(&s.DirtyRectangle, bounds); rerender {
		items := allItems
		if len(items) == 0 {
			rl.EndTextureMode() //StartDirtyRectangle does a rl.StartTextureMode()
			return
		}
		if s.FilterSLE != nil && len(s.FilterSLE.Text) > 0 {
			items = s.FilterItems
		}
		if IsFilterActive(*s) {
			filterBounds := CutTop(&textureBounds, font.Size)
			labelBounds := CutLeft(&filterBounds, MeasureText(font, label_text).X)
			DrawTextBox(font, label_text, labelBounds, TEXT_LEFT, theme.Filter)
			DrawBox(filterBounds, theme.Filter.Thickness, theme.Filter.Bg, theme.Filter.Border)
			filterLabelPos := rl.Vector2{filterBounds.X + font.WhiteSpaceWidth, filterBounds.Y}
			BeginScissorMode(filterBounds)
			DrawSLEEx(font, s.FilterSLE, filterLabelPos, theme.SLE, s.FilterInputActive, dt)
			rl.EndScissorMode()
		}
		s.MaxDisplayableItems = int(Floor(textureBounds.Height / (s.ItemHeight + s.GapY)))
		yRemainder := (textureBounds.Height - (s.ItemHeight+s.GapY)*float32(s.MaxDisplayableItems)) / 2
		s.TotalItems = len(items)
		if s.MaxDisplayableItems < s.TotalItems {
			scrollBar := CutRight(&textureBounds, theme.PuckThickness)
			DrawBoxEx(scrollBar, theme.BoxTheme)
			maxHorizontalItems := 1
			puckOffset := float32(s.Cursor*maxHorizontalItems) / float32(s.TotalItems) * (scrollBar.Height - theme.MinCursorPuckSize)
			puckRec := GetPuck(scrollBar, s.OffsetY, s.MaxDisplayableItems, s.TotalItems, maxHorizontalItems, theme.MinPuckSize)
			rl.DrawRectangleRec(Pad(puckRec, theme.Thickness), theme.Puck)

			for i := range items {
				_, selected := selectedIndexes[i]
				if selected {
					selectedPuckOffset := float32(i*maxHorizontalItems) / float32(s.TotalItems) * (scrollBar.Height - theme.MinCursorPuckSize)
					selectedCursorRec := rl.Rectangle{
						scrollBar.X,
						scrollBar.Y + selectedPuckOffset,
						scrollBar.Width,
						theme.MinCursorPuckSize,
					}
					rl.DrawRectangleRec(Pad(selectedCursorRec, theme.Thickness), theme.SelectedCursorPuck)
				}
			}
			cursorRec := rl.Rectangle{
				scrollBar.X,
				scrollBar.Y + puckOffset,
				scrollBar.Width,
				theme.MinCursorPuckSize,
			}
			rl.DrawRectangleRec(Pad(cursorRec, theme.Thickness), theme.CursorPuck)
		}
		cursor := GetCursor(s)
		BeginScissorMode(textureBounds)
		max := Min(s.OffsetY+s.MaxDisplayableItems-1, len(items)-1)
		s.Hitboxes = make([]rl.Rectangle, max+1)
		for i := s.OffsetY; i <= max; i++ {
			item := items[i]
			rec := rl.Rectangle{
				textureBounds.X + theme.Thickness + s.PaddingX,
				textureBounds.Y + yRemainder + theme.Thickness + (s.ItemHeight+s.GapY)*float32(i-s.OffsetY),
				textureBounds.Width - theme.Thickness*2 - s.PaddingX*2,
				s.ItemHeight,
			}
			hitbox := rl.Rectangle{rec.X + bounds.X, rec.Y + bounds.Y, rec.Width, rec.Height}
			s.Hitboxes[i-s.OffsetY] = hitbox
			_, selected := selectedIndexes[i]
			drawItem(rec, i, item, selected, i == cursor)
		}
		rl.EndScissorMode()
	}
	EndDirtyRectangle(&s.DirtyRectangle, bounds)
}

type DirtyRectangle struct {
	Dirty      bool
	PrevBounds rl.Rectangle
	Texture    rl.RenderTexture2D
}

func StartDirtyRectangle(d *DirtyRectangle, bounds rl.Rectangle) (_rerender bool, _textureBounds rl.Rectangle) {
	textureBounds := rl.Rectangle{0, 0, bounds.Width, bounds.Height}
	if !RectangleEquals(d.PrevBounds, bounds) {
		d.Dirty = true
		d.PrevBounds = bounds
		if d.Texture.ID > 0 {
			rl.UnloadRenderTexture(d.Texture)
		}
		d.Texture = rl.LoadRenderTexture(int32(bounds.Width), int32(bounds.Height))
		rl.BeginTextureMode(d.Texture)
		rl.ClearBackground(rl.Blank)
		return true, textureBounds
	} else if d.Dirty {
		rl.BeginTextureMode(d.Texture)
		rl.ClearBackground(rl.Blank)
		return true, textureBounds
	}
	return false, textureBounds
}
func EndDirtyRectangle(d *DirtyRectangle, bounds rl.Rectangle) {
	if d.Dirty {
		rl.EndTextureMode()
		d.Dirty = false
	}
	rec := rl.Rectangle{0, 0, bounds.Width, -bounds.Height}
	pos := rl.Vector2{bounds.X, bounds.Y}
	rl.DrawTextureRec(d.Texture.Texture, rec, pos, rl.White)
}
func EndDirtyRectangleEx(d *DirtyRectangle, bounds rl.Rectangle) (_rec rl.Rectangle, _wasModified bool) {
	wasModified := false
	if d.Dirty {
		rl.EndTextureMode()
		d.Dirty = false
		wasModified = true
	}
	rec := rl.Rectangle{0, 0, bounds.Width, -bounds.Height}
	return rec, wasModified
}

var SCROLL_KEYBINDINGS = []KeyBind{
	{SCROLL_SELECT_CURSOR_UP, MOD_SHIFT | rl.KeyUp},
	{SCROLL_SELECT_CURSOR_DOWN, MOD_SHIFT | rl.KeyDown},
	{SCROLL_CURSOR_DOWN, rl.KeyDown},
	{SCROLL_SELECT_PAGE_DOWN, MOD_SHIFT | rl.KeyPageDown},
	{SCROLL_CURSOR_PAGE_DOWN, rl.KeyPageDown},
	{SCROLL_SELECT_PAGE_UP, MOD_SHIFT | rl.KeyPageUp},
	{SCROLL_CURSOR_PAGE_UP, rl.KeyPageUp},
	{SCROLL_SELECT_HOME, MOD_SHIFT | rl.KeyHome},
	{SCROLL_CURSOR_HOME, rl.KeyHome},
	{SCROLL_SELECT_END, MOD_SHIFT | rl.KeyEnd},
	{SCROLL_CURSOR_END, rl.KeyEnd},
	{SCROLL_CURSOR_UP, rl.KeyUp},
	{SCROLL_SELECT_ALL, MOD_CTRL | rl.KeyA},
	{SCROLL_SELECT, rl.KeySpace},
	{SCROLL_FILTER, MOD_CTRL | rl.KeyF},
	{SCROLL_CANCEL, rl.KeyEscape},
	{SCROLL_CENTER_TO_CURSOR, MOD_CTRL | rl.KeyHome},
}

func GetCursor[T comparable](s *Scroll[T]) int {
	if IsFilterActive(*s) {
		return s.FilterCursor
	}
	return s.Cursor
}
func SetCursor[T comparable](s *Scroll[T], new int) {
	if IsFilterActive(*s) {
		s.FilterCursor = new
		return
	}
	s.Cursor = new
}
func GetHighlightedScrollItem[T comparable](s Scroll[T], allItems []T) T {
	var item T
	if IsFilterActive(s) {
		if len(s.FilterItems) > 0 {
			item = s.FilterItems[s.FilterCursor]
		}
		return item
	}
	if len(allItems) > 0 {
		item = allItems[s.Cursor]
	}
	return item
}
func CenterCursor[T comparable](s *Scroll[T]) {
	max := s.MaxDisplayableItems + 1
	cursor := s.Cursor
	if IsFilterActive(*s) {
		cursor = s.FilterCursor
	}
	if cursor < s.OffsetY {
		s.OffsetY = Max(0, cursor-(max/2))
	} else if cursor > s.OffsetY+s.MaxDisplayableItems-1 {
		t := Max(s.TotalItems-s.MaxDisplayableItems-1, 0)
		s.OffsetY = Min(cursor-(max/2), t)
	}
}

func ScrollInput[T comparable](
	input *Input,
	bounds rl.Rectangle,
	s *Scroll[T],
	allItems []T,
	selectedIndexes *map[int]struct{},
	filterFunc func(query string, items []T) (_filteredItems []T, _indexes []int),
) {
	if s.FilterSLE != nil && s.FilterInputActive {
		sleResult := s.FilterSLE.Input(input, DEFAULT_SLE_KEYBINDINGS)
		switch sleResult {
		case SLE_R_HAD_CHAR_INPUT:
			s.Dirty = true
			fallthrough
		case SLE_R_NO_INPUT:
			//user still has control over some scroll input here
			if len(s.FilterSLE.Text) > 0 && s.PreviousFilterValue != string(s.FilterSLE.Text) {
				s.PreviousFilterValue = string(s.FilterSLE.Text)
				s.FilterItems, s.FilterIndexes = filterFunc(string(s.FilterSLE.Text), allItems)
				max := len(s.FilterItems) - s.MaxDisplayableItems - 1
				if max < 0 {
					s.FilterCursor = 0
				} else {
					s.FilterCursor = Clamp(s.FilterCursor, 0, len(s.FilterItems)-s.MaxDisplayableItems-1)
				}
			}
		case SLE_R_HAD_INPUT:
			s.Dirty = true
		case SLE_R_ACCEPT:
			s.Dirty = true
			//accept the filter, let user have full control over scroll
			s.FilterInputActive = false
		case SLE_R_QUIT:
			s.Dirty = true
			//we cancel out of the filtering, showing all files
			if len(s.FilterIndexes) > 0 { //go to the file that the filter cursor was on
				itemIndex := s.FilterIndexes[s.FilterCursor]
				s.Cursor = itemIndex
			}
			s.FilterInputActive = false
			s.FilterSLE.Reset()
			s.FilterItems = nil
			s.FilterIndexes = nil
			CenterCursor(s)
		}
		return
	}
	items := allItems
	if s.FilterSLE != nil && len(s.FilterSLE.Text) > 0 {
		items = s.FilterItems
	}
	action := input.CheckAction(SCROLL_KEYBINDINGS, true)
	mousePos := rl.GetMousePosition()
	if rl.CheckCollisionPointRec(mousePos, bounds) {
		wheelY := rl.GetMouseWheelMove()
		amount := 1
		if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
			amount += 5
		}
		if wheelY > 0 {
			s.OffsetY = Clamp(s.OffsetY-amount, 0, len(allItems))
			s.Dirty = true
			return
		} else if wheelY < 0 {
			if len(allItems) > s.MaxDisplayableItems &&
				len(allItems)-s.MaxDisplayableItems > s.OffsetY {
				s.OffsetY = Clamp(s.OffsetY+amount, 0, len(allItems))
				s.Dirty = true
				return
			}
		}
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			for i, h := range s.Hitboxes {
				if rl.CheckCollisionPointRec(mousePos, h) {
					SetCursor(s, s.OffsetY+i)
					s.Dirty = true
					return
				}
			}
		}
	}
	cursor := GetCursor(s)
	switch action {
	case SCROLL_SELECT_CURSOR_UP:
		if selectedIndexes != nil {
			(*selectedIndexes)[cursor] = struct{}{}
		}
		newPos := Clamp(cursor-1, 0, len(items)-1)
		SetCursor(s, newPos)
		if selectedIndexes != nil {
			(*selectedIndexes)[newPos] = struct{}{}
		}
	case SCROLL_SELECT_CURSOR_DOWN:
		if selectedIndexes != nil {
			(*selectedIndexes)[cursor] = struct{}{}
		}
		newPos := Clamp(cursor+1, 0, len(items)-1)
		SetCursor(s, newPos)
		if selectedIndexes != nil {
			(*selectedIndexes)[newPos] = struct{}{}
		}
	case SCROLL_SELECT_PAGE_DOWN:
		amount := s.MaxDisplayableItems - 1
		newPos := Clamp(cursor+amount, 0, len(items)-1)
		if selectedIndexes != nil {
			for i := cursor; i <= newPos; i++ {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
		SetCursor(s, newPos)
	case SCROLL_SELECT_PAGE_UP:
		amount := s.MaxDisplayableItems - 1
		newPos := Clamp(cursor-amount, 0, len(items)-1)
		if selectedIndexes != nil {
			for i := cursor; i >= newPos; i-- {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
		SetCursor(s, newPos)
	case SCROLL_SELECT_HOME:
		if selectedIndexes != nil {
			for i := cursor; i >= 0; i-- {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
		SetCursor(s, 0)
		s.OffsetY = 0
	case SCROLL_SELECT_END:
		if selectedIndexes != nil {
			for i := cursor; i < len(items); i++ {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
		SetCursor(s, len(items)-1)
		s.OffsetY = Max(len(items)-s.MaxDisplayableItems, 0)
	case SCROLL_SELECT:
		if selectedIndexes != nil {
			if _, ok := (*selectedIndexes)[cursor]; ok {
				delete((*selectedIndexes), cursor)
			} else {
				(*selectedIndexes)[cursor] = struct{}{}
			}
		}
	case SCROLL_SELECT_ALL:
		if selectedIndexes != nil {
			for i := range items {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
	case SCROLL_CURSOR_UP:
		newPos := NotShitMod(cursor-1, len(items))
		SetCursor(s, newPos)
	case SCROLL_CURSOR_DOWN:
		newPos := NotShitMod(cursor+1, len(items))
		SetCursor(s, newPos)
	case SCROLL_CURSOR_PAGE_UP:
		amount := s.MaxDisplayableItems - 1
		newPos := Clamp(cursor-amount, 0, len(items)-1)
		SetCursor(s, newPos)
	case SCROLL_CURSOR_PAGE_DOWN:
		amount := s.MaxDisplayableItems - 1
		newPos := Clamp(cursor+amount, 0, len(items)-1)
		SetCursor(s, newPos)
	case SCROLL_CURSOR_HOME:
		SetCursor(s, 0)
		s.OffsetY = 0
	case SCROLL_CURSOR_END:
		SetCursor(s, len(items)-1)
		s.OffsetY = Max(len(items)-s.MaxDisplayableItems, 0)
	case SCROLL_FILTER:
		s.FilterInputActive = !s.FilterInputActive
		if s.FilterSLE != nil && len(s.FilterSLE.Text) == 0 {
			s.FilterCursor = 0
		}
		s.Dirty = true
		if IsFilterActive(*s) {
			s.PreviousFilterValue = string(s.FilterSLE.Text)
			s.FilterItems, s.FilterIndexes = filterFunc(string(s.FilterSLE.Text), allItems)
			max := len(s.FilterItems) - s.MaxDisplayableItems - 1
			if max < 0 {
				s.FilterCursor = 0
			} else {
				s.FilterCursor = Clamp(s.FilterCursor, 0, len(s.FilterItems)-s.MaxDisplayableItems-1)
			}
		} else {
			s.FilterItems = allItems
		}
	case SCROLL_CANCEL:
		if s.FilterSLE != nil && len(s.FilterSLE.Text) > 0 {
			if len(s.FilterIndexes) > 0 {
				itemIndex := s.FilterIndexes[s.FilterCursor]
				s.Cursor = itemIndex
				CenterCursor(s)
			}
			s.FilterInputActive = false
			s.FilterSLE.Reset()
			s.FilterItems = nil
		} else {
			if selectedIndexes != nil {
				if len((*selectedIndexes)) == 0 {
					input.Keys[rl.KeyEscape] = true
				} else {
					(*selectedIndexes) = make(map[int]struct{})
				}
			}
		}
	case SCROLL_CENTER_TO_CURSOR:
		CenterCursor(s)
	}
	if action != NONE {
		cursor := GetCursor(s)
		s.Dirty = true
		if cursor < s.OffsetY {
			s.OffsetY = cursor
		} else if cursor > s.OffsetY+s.MaxDisplayableItems-1 {
			s.OffsetY = cursor - s.MaxDisplayableItems + 1
		}
	}
}

var SCROLL_UNSELECTABLE_KEYBINDINGS = []KeyBind{
	{SCROLL_CURSOR_DOWN, rl.KeyDown},
	{SCROLL_CURSOR_PAGE_DOWN, rl.KeyPageDown},
	{SCROLL_CURSOR_PAGE_UP, rl.KeyPageUp},
	{SCROLL_CURSOR_HOME, rl.KeyHome},
	{SCROLL_CURSOR_END, rl.KeyEnd},
	{SCROLL_CURSOR_UP, rl.KeyUp},
	{SCROLL_FILTER, MOD_CTRL | rl.KeyF},
	{SCROLL_CANCEL, rl.KeyEscape},
}

type VariableScroll[T interface {
	VariableScrollItem
	comparable
}] struct {
	Cursor     int
	Velocity   float32
	GapY       float32
	PaddingX   float32
	PaddingY   float32
	FullHeight float32
	OffsetY    float32

	PuckThickness float32

	FilterSLE           *SingleLineEditor
	PreviousFilterValue string
	IsFiltering         bool
	FilterItems         []T
	FilterIndexes       []int
	FilterCursor        int

	Hitboxes []rl.Rectangle

	DirtyRectangle
}

func MoveCursorVariable[T interface {
	VariableScrollItem
	comparable
}](
	cursor *int, offsetY *float32, gapY float32, bounds rl.Rectangle, items []T, val, min, max int,
) bool {
	if val == 0 {
		return false
	}
	newCursor := Clamp(*cursor+val, 0, max)
	if newCursor == *cursor {
		return false
	}
	accumulatedHeight := float32(0)
	for i, it := range items {
		if i > *cursor {
			break
		}
		accumulatedHeight += it.GetHeight() + gapY
	}
	if val < 0 {
		prevItemHeight := items[*cursor].GetHeight()
		*cursor = newCursor
		newItemHeight := items[*cursor].GetHeight()
		if *offsetY != 0 {
			diff := Abs(*offsetY) - (accumulatedHeight - newItemHeight - prevItemHeight - gapY*2)
			if diff > 0 {
				*offsetY += diff
			}
		}
		return true
	} else {
		*cursor = newCursor
		newItemHeight := items[*cursor].GetHeight()
		if accumulatedHeight+*offsetY+newItemHeight > bounds.Height {
			*offsetY -= newItemHeight - (Abs(*offsetY) + bounds.Height - accumulatedHeight)
		}
		return true
	}
}
func MoveCursorWrapVariable(cursor *int, val, min, max int) bool {
	if val != 0 {
		newPos := NotShitMod(*cursor+val, max+1)
		*cursor = newPos
		return true
	}
	return false
}

type VariableScrollItem interface {
	GetHeight() float32
}

// todo: messy, but works. Could be simpler
func VariableScrollFilterInput[T interface {
	VariableScrollItem
	comparable
}](
	input *Input,
	scroll *VariableScroll[T],
	allItems []T,
	filterFunc func(filter string, item T) bool,
) []T {
	sleResult := scroll.FilterSLE.Input(input, DEFAULT_SLE_KEYBINDINGS)
	switch sleResult {
	case SLE_R_NO_INPUT, SLE_R_HAD_CHAR_INPUT:
		if len(scroll.FilterSLE.Text) > 0 {
			filteredItems := make([]T, 0, len(allItems))
			for _, c := range allItems {
				if filterFunc(string(scroll.FilterSLE.Text), c) {
					filteredItems = append(filteredItems, c)
				}
			}
			return filteredItems
		}
	case SLE_R_ACCEPT:
		scroll.IsFiltering = false
		if len(scroll.FilterSLE.Text) > 0 {
			return scroll.FilterItems
		}
	case SLE_R_QUIT:
		scroll.IsFiltering = false
		scroll.FilterSLE.Reset()
		scroll.FilterItems = nil
	}
	return allItems
}

// todo: could be optimized to calculate height only once/when changes happen
func DrawVariableScroll[T interface {
	VariableScrollItem
	comparable
}](
	font Font,
	bounds rl.Rectangle,
	s *VariableScroll[T],
	allItems []T,
	selectedIndexes map[int]struct{},
	theme ScrollTheme,
	dt float32,
	drawItem func(itemBounds rl.Rectangle, index int, item T, selected, highlighted bool),
) {
	const (
		label_text = "Filter: "
	)

	if rerender, textureBounds := StartDirtyRectangle(&s.DirtyRectangle, bounds); rerender {
		items := allItems
		if len(items) == 0 {
			rl.EndTextureMode() //StartDirtyRectangle does a rl.StartTextureMode()
			return
		}
		if len(s.FilterSLE.Text) > 0 {
			items = s.FilterItems
		}
		pos := rl.Vector2{bounds.X, bounds.Y + s.OffsetY}
		s.FullHeight = float32(0)
		showScrollbar := false
		accumulatedHeight := float32(0)
		{
			for _, it := range items {
				accumulatedHeight += it.GetHeight() + s.GapY
				if !showScrollbar && accumulatedHeight > textureBounds.Height {
					showScrollbar = true
				}
			}
		}
		s.FullHeight = accumulatedHeight + s.PaddingY
		var scrollBar rl.Rectangle
		if showScrollbar {
			scrollBar = CutRight(&textureBounds, s.PuckThickness)
			DrawBox(scrollBar, theme.Thickness, theme.Bg, theme.Border)
			ratio := textureBounds.Height / s.FullHeight
			puck_height := textureBounds.Height * ratio
			puck_rec := rl.Rectangle{
				scrollBar.X + theme.Thickness,
				scrollBar.Y - ratio*s.OffsetY + theme.Thickness,
				scrollBar.Width - theme.Thickness*2,
				puck_height,
			}
			rl.DrawRectangleRec(puck_rec, theme.Puck)
		}
		DrawBox(textureBounds, theme.Thickness, theme.Bg, theme.Border)

		BeginScissorMode(Pad(textureBounds, theme.Thickness))
		for i, it := range items {
			rec := rl.Rectangle{
				pos.X + theme.Thickness + s.PaddingX,
				pos.Y + theme.Thickness + s.PaddingY,
				textureBounds.Width - theme.Thickness*2 - s.PaddingX*2,
				it.GetHeight(),
			}
			if rl.CheckCollisionRecs(rec, textureBounds) {
				_, selected := selectedIndexes[i]
				drawItem(rec, i, it, selected, i == s.Cursor)
			}
			pos.Y += it.GetHeight() + s.GapY
		}
		rl.EndScissorMode()
	}
	EndDirtyRectangle(&s.DirtyRectangle, bounds)
}

func VariableScrollInput[T interface {
	VariableScrollItem
	comparable
}](
	input *Input,
	s *VariableScroll[T],
	bounds rl.Rectangle,
	allItems []T,
	selectedIndexes *map[int]struct{},
) {
	items := allItems
	if s.FilterSLE != nil && len(s.FilterSLE.Text) > 0 {
		items = s.FilterItems
	}
	action := input.CheckAction(SCROLL_KEYBINDINGS, true)
	if action != NONE {
		s.Dirty = true
	}
	switch action {
	case SCROLL_SELECT_CURSOR_UP:
		if selectedIndexes != nil {
			(*selectedIndexes)[s.Cursor] = struct{}{}
		}
		MoveCursorVariable(&s.Cursor, &s.OffsetY, s.GapY, bounds, items, -1, 0, len(items)-1)
		if selectedIndexes != nil {
			(*selectedIndexes)[s.Cursor] = struct{}{}
		}
	case SCROLL_SELECT_CURSOR_DOWN:
		if selectedIndexes != nil {
			(*selectedIndexes)[s.Cursor] = struct{}{}
		}
		MoveCursorVariable(&s.Cursor, &s.OffsetY, s.GapY, bounds, items, +1, 0, len(items)-1)
		if selectedIndexes != nil {
			(*selectedIndexes)[s.Cursor] = struct{}{}
		}
	case SCROLL_SELECT_PAGE_DOWN:
		// newPos := Clamp(s.Cursor+s.FullHeight-1, 0, len(items)-1)
		// for i := s.Cursor; i <= newPos; i++ {
		// 	items[i].SetSelected(true)
		// }
		// s.MoveCursor(s.FullHeight-1, 0, len(items)-1)
		// s.SelectedAmount = len(selectedIndexes)
	case SCROLL_SELECT_PAGE_UP:
		// newPos := Clamp(s.Cursor-s.FullHeight-1, 0, len(items)-1)
		// for i := s.Cursor; i >= newPos; i-- {
		// 	items[i].SetSelected(true)
		// }
		// s.MoveCursor(-(s.FullHeight - 1), 0, len(items)-1)
		// s.SelectedAmount = len(selectedIndexes)
	case SCROLL_SELECT_HOME:
		if selectedIndexes != nil {
			for i := s.Cursor; i >= 0; i-- {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
		s.Cursor = 0
		s.OffsetY = 0
	case SCROLL_SELECT_END:
		if selectedIndexes != nil {
			for i := s.Cursor; i < len(items); i++ {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
		s.Cursor = len(items) - 1
		s.OffsetY = -s.FullHeight + bounds.Height
	case SCROLL_SELECT:
		if selectedIndexes != nil {
			if _, ok := (*selectedIndexes)[s.Cursor]; ok {
				delete((*selectedIndexes), s.Cursor)
			} else {
				(*selectedIndexes)[s.Cursor] = struct{}{}
			}
		}
	case SCROLL_SELECT_ALL:
		if selectedIndexes != nil {
			for i := range items {
				(*selectedIndexes)[i] = struct{}{}
			}
		}
	case SCROLL_CURSOR_UP:
		MoveCursorVariable(&s.Cursor, &s.OffsetY, s.GapY, bounds, items, -1, 0, len(items)-1)
	case SCROLL_CURSOR_DOWN:
		MoveCursorVariable(&s.Cursor, &s.OffsetY, s.GapY, bounds, items, +1, 0, len(items)-1)
	case SCROLL_CURSOR_PAGE_UP:
		//s.MoveCursor(-(s.FullHeight - 1), 0, len(items)-1)
	case SCROLL_CURSOR_PAGE_DOWN:
		//s.MoveCursor(s.FullHeight-1, 0, len(items)-1)
	case SCROLL_CURSOR_HOME:
		s.Cursor = 0
		s.OffsetY = 0
	case SCROLL_CURSOR_END:
		s.Cursor = len(items) - 1
		s.OffsetY = -s.FullHeight + bounds.Height
	case SCROLL_FILTER:
		s.IsFiltering = !s.IsFiltering
	case SCROLL_CANCEL:
		if len(s.FilterSLE.Text) > 0 {
			s.IsFiltering = false
			s.FilterSLE.Reset()
			s.FilterItems = nil
		} else {
			if selectedIndexes != nil {
				if len(*selectedIndexes) == 0 {
					input.Keys[rl.KeyEscape] = true
				} else {
					(*selectedIndexes) = make(map[int]struct{})
				}
			}
		}
	}
}

type GridScroll[T interface {
	VariableScrollItem
	comparable
}] struct {
	Cursor              int
	OffsetY             int
	GapX                float32
	GapY                float32
	TotalItems          int
	MaxDisplayableItems int
	MaxHorizontalItems  int
	Selected_amount     int
	Filter              *ScrollFilter[T]

	PuckThickness float32

	DirtyRectangle
}

func DrawGridScroll[T interface {
	VariableScrollItem
	comparable
}](
	font Font,
	bounds rl.Rectangle,
	s *GridScroll[T],
	allItems []T,
	itemDimensions rl.Vector2,
	theme ScrollTheme,
	dt float32,
	drawItem func(itemBounds rl.Rectangle, index int, item T, onCursor bool),
) {
	DrawScrollFilter(font, &bounds, s.Filter, theme, dt)

	if rerender, textureBounds := StartDirtyRectangle(&s.DirtyRectangle, bounds); rerender {
		items := allItems
		if s.Filter != nil && len(s.Filter.Items) > 0 {
			items = s.Filter.Items
		}
		s.MaxHorizontalItems = int(Floor(textureBounds.Width / (itemDimensions.X + s.GapX)))
		if s.MaxHorizontalItems == 0 {
			s.MaxHorizontalItems = 1
		}
		left_offset := (textureBounds.Width - float32(s.MaxHorizontalItems)*(itemDimensions.X+s.GapX)) / 2
		s.MaxDisplayableItems = s.MaxHorizontalItems * int(Floor(textureBounds.Height/(s.GapY+(itemDimensions.Y+s.GapY))))
		s.TotalItems = len(items)
		if s.MaxDisplayableItems < s.TotalItems {
			scrollBar := CutRight(&textureBounds, s.PuckThickness)
			DrawBox(scrollBar, theme.Thickness, theme.Bg, theme.Border)
			puck_rec := GetPuck(scrollBar, s.OffsetY, s.MaxDisplayableItems, s.TotalItems, s.MaxHorizontalItems, 1)
			rl.DrawRectangleRec(Pad(puck_rec, theme.Thickness), theme.Puck)
		}
		BeginScissorMode(Pad(textureBounds, theme.Thickness))
		col := 0
		row := -1
		for i, it := range items {
			if i%s.MaxHorizontalItems == 0 {
				row += 1
				col = 0
			} else {
				col += 1
			}
			if row < s.OffsetY || row > s.OffsetY+s.MaxDisplayableItems-1 {
				continue
			}
			rec := rl.Rectangle{
				left_offset + textureBounds.X + theme.Thickness + (itemDimensions.X+s.GapX)*float32(col),
				textureBounds.Y + s.GapY + theme.Thickness + (itemDimensions.Y+s.GapY)*float32(row-s.OffsetY),
				itemDimensions.X,
				itemDimensions.Y,
			}
			drawItem(rec, i, it, i == s.Cursor)
		}
		rl.EndScissorMode()
	}
	EndDirtyRectangle(&s.DirtyRectangle, bounds)
}

// unfinished
func DrawTreeScroll[T string](
	font Font,
	bounds rl.Rectangle,
	s *Scroll[string],
	allItems []string,
	expandedIndexes map[int]struct{},
	selectedIndexes map[int]struct{},
	theme ScrollTheme,
	dt float32,
	drawItem func(itemBounds rl.Rectangle, index int, item string, selected, highlighted bool),
) {
	var expand_icon_radius = font.Em(0.25)

	if rerender, textureBounds := StartDirtyRectangle(&s.DirtyRectangle, bounds); rerender {
		//currentExpanded:=
		// item := make([]string, 0, len(allItems))
		// for i, v := range allItems {
		// 	_, ok := expandedIndexes[i]
		// 	if v{
		// 		item
		// 	}
		// }
		items := allItems
		if len(items) == 0 {
			rl.EndTextureMode() //StartDirtyRectangle does a rl.StartTextureMode()
			return
		}

		s.MaxDisplayableItems = int(Floor(textureBounds.Height / (s.ItemHeight + s.GapY)))
		yRemainder := (textureBounds.Height - (s.ItemHeight+s.GapY)*float32(s.MaxDisplayableItems)) / 2
		s.TotalItems = len(items)
		if s.MaxDisplayableItems < s.TotalItems {
			scrollBar := CutRight(&textureBounds, theme.PuckThickness)
			DrawBox(scrollBar, theme.Thickness, theme.Bg, theme.Border)
			maxHorizontalItems := 1
			puckOffset := float32(s.Cursor*maxHorizontalItems) / float32(s.TotalItems) * (scrollBar.Height - theme.MinCursorPuckSize)
			cursorRec := rl.Rectangle{scrollBar.X, scrollBar.Y + puckOffset, scrollBar.Width, theme.MinCursorPuckSize}
			puckRec := GetPuck(scrollBar, s.OffsetY, s.MaxDisplayableItems, s.TotalItems, maxHorizontalItems, theme.MinPuckSize)
			rl.DrawRectangleRec(Pad(puckRec, theme.Thickness), theme.Puck)
			rl.DrawRectangleRec(Pad(cursorRec, theme.Thickness), rl.Blue)
		}
		cursor := GetCursor(s)
		BeginScissorMode(textureBounds)
		max := Min(s.OffsetY+s.MaxDisplayableItems-1, len(items)-1)
		s.Hitboxes = make([]rl.Rectangle, max+1)
		startingOffsets := rl.Rectangle{
			textureBounds.X + theme.Thickness + s.PaddingX,
			textureBounds.Y + yRemainder + theme.Thickness,
			textureBounds.Width - theme.Thickness*2 - s.PaddingX*2,
			s.ItemHeight,
		}

		for i := s.OffsetY; i <= max; i++ {
			item := items[i]
			str, _ := strings.CutSuffix(item, "/")
			tabAmount := strings.Count(str, "/")
			prevTabAmount := 0
			if i > 0 {
				prev_item := items[i-1]
				prevTabAmount = strings.Count(prev_item, "/")
			}

			rec := startingOffsets
			rec.Y += (s.GapY + s.ItemHeight) * float32(i-s.OffsetY)
			hitbox := rl.Rectangle{rec.X + bounds.X, rec.Y + bounds.Y, rec.Width, rec.Height}
			color := rl.White

			// for k, v := range allItems {

			// }
			for t := range tabAmount {
				{ //lines
					if i > 0 { //if it's the first item, we don't need to do anything? I think it's guaranteed
						if t > 0 && t == tabAmount-1 {
							negative_height := s.ItemHeight - (s.ItemHeight / 2) + s.GapY
							if prevTabAmount != tabAmount { //first elbow
								negative_height = s.ItemHeight - (s.ItemHeight/2 + expand_icon_radius) + s.GapY
							}
							positive_height := s.ItemHeight / 2
							width := s.TreeTabSize - expand_icon_radius
							posA := rl.Vector2{rec.X, rec.Y - negative_height}
							posB := rl.Vector2{posA.X, rec.Y + positive_height}
							posC := rl.Vector2{rec.X + width, posB.Y}
							rl.DrawLineV(posA, posB, color)
							rl.DrawLineV(posB, posC, color)
						} else if t > 0 {
							negative_height := s.ItemHeight/2 + s.GapY
							posA := rl.Vector2{rec.X, rec.Y - negative_height}
							posB := rl.Vector2{rec.X, rec.Y + rec.Height}
							rl.DrawLineV(posA, posB, color)
						}
					}
				}

				rec.X += s.TreeTabSize

				{ //balls
					if t == tabAmount-1 {
						expandIconPos := rl.Vector2{rec.X, rec.Y + rec.Height/2}
						_, exp := expandedIndexes[i]
						if exp {
							rl.DrawCircleV(expandIconPos, expand_icon_radius, rl.White)
						} else {
							rl.DrawCircleLinesV(expandIconPos, expand_icon_radius, rl.White)
						}
						rec.X += font.Em(0.5)
					}
				}
			}

			s.Hitboxes[i-s.OffsetY] = hitbox
			_, selected := selectedIndexes[i]

			drawItem(rec, i, item, selected, i == cursor)
		}
		rl.EndScissorMode()
	}
	EndDirtyRectangle(&s.DirtyRectangle, bounds)
}

type ScrollingTextDirection uint8

const (
	RIGHT_TO_LEFT ScrollingTextDirection = iota
	LEFT_TO_RIGHT
)

type ScrollingTextData struct {
	Pos         rl.Vector2
	Text        string
	Speed       float32
	Direction   ScrollingTextDirection
	InitialWait float32

	DirtyRectangle
}

// todo: not really satisfied with this, maybe it's better to have a flag to know when it's the first
// render, this way it could actually start always at bounds.X, wait the InitialWait, then start moving.
func DrawScrollingText(
	font Font,
	st *ScrollingTextData,
	bounds rl.Rectangle,
	theme BoxTextTheme,
	dt float32,
) {
	width := MeasureText(font, st.Text).X
	st.Pos.Y = bounds.Y + (bounds.Height-font.Size)/2
	step := st.Speed * dt
	halfStep := step / 2
	switch st.Direction {
	case RIGHT_TO_LEFT:
		if st.InitialWait > 0 {
			st.InitialWait -= dt
		} else {
			if st.Pos.X < bounds.X-width || st.Pos.X > bounds.X+bounds.Width+width {
				st.Pos.X = bounds.X + bounds.Width
			} else {
				st.Pos.X -= step
			}
			if st.Pos.X <= bounds.X+halfStep && st.Pos.X >= bounds.X-halfStep {
				st.InitialWait = SCROLLING_TEXT_INITIAL_WAIT
			}
		}
	case LEFT_TO_RIGHT:
		if st.InitialWait > 0 {
			st.InitialWait -= dt
		} else {
			if st.Pos.X < bounds.X-width || st.Pos.X > bounds.X+bounds.Width {
				st.Pos.X = bounds.X - width
			} else {
				st.Pos.X += step
			}
			if st.Pos.X <= bounds.X+halfStep && st.Pos.X >= bounds.X-halfStep {
				st.InitialWait = SCROLLING_TEXT_INITIAL_WAIT
			}
		}
	}
	if rerender, textureBounds := StartDirtyRectangle(&st.DirtyRectangle, bounds); rerender {
		pos := rl.Vector2{textureBounds.X, textureBounds.Y}
		DrawText(font, pos, st.Text, theme.Text)
	}
	rec, _ := EndDirtyRectangleEx(&st.DirtyRectangle, bounds)
	BeginScissorMode(bounds)
	rl.DrawTextureRec(st.Texture.Texture, rec, st.Pos, rl.White)
	rl.EndScissorMode()
}

type Action int

const (
	NONE Action = iota
	QUIT
	CLICK
	CANCEL

	SCROLL_CURSOR_DOWN
	SCROLL_CURSOR_PAGE_DOWN
	SCROLL_CURSOR_PAGE_UP
	SCROLL_CURSOR_HOME
	SCROLL_CURSOR_END
	SCROLL_CURSOR_UP
	SCROLL_FILTER
	SCROLL_CANCEL
	SCROLL_SELECT_CURSOR_UP
	SCROLL_SELECT_CURSOR_DOWN
	SCROLL_SELECT_PAGE_DOWN
	SCROLL_SELECT_PAGE_UP
	SCROLL_SELECT_HOME
	SCROLL_SELECT_END
	SCROLL_SELECT
	SCROLL_SELECT_ALL
	SCROLL_CENTER_TO_CURSOR

	SLE_LEFT
	SLE_RIGHT
	SLE_LEFT_WORD
	SLE_RIGHT_WORD
	SLE_LINE_START
	SLE_LINE_END
	SLE_DOC_START
	SLE_DOC_END

	SLE_DELETE_RIGHT      //undo: until action changes
	SLE_DELETE_WORD_RIGHT //undo: each removal
	SLE_DELETE_RIGHT_ALL  //undo: each removal
	SLE_DELETE_LEFT       //undo: until action changes
	SLE_DELETE_WORD_LEFT  //undo: each removal
	SLE_DELETE_LEFT_ALL   //undo: each removal
	SLE_CUT               //undo: each removal
	SLE_COPY
	SLE_PASTE
	SLE_CYCLE_CASES

	SLE_SELECT_LEFT
	SLE_SELECT_RIGHT
	SLE_SELECT_LEFT_WORD
	SLE_SELECT_RIGHT_WORD
	SLE_SELECT_LINE_START
	SLE_SELECT_DOC_START
	SLE_SELECT_LINE_END
	SLE_SELECT_DOC_END
	SLE_SELECT_ALL

	SLE_UNDO
	SLE_REDO
	SLE_ACCEPT_FILTER
	SLE_AUTOCOMPLETE
	SLE_QUIT

	LAST_ACTION
)

var ACTION_STR = []string{"NONE", "QUIT", "CLICK", "CANCEL", "SCROLL_CURSOR_DOWN", "SCROLL_CURSOR_PAGE_DOWN", "SCROLL_CURSOR_PAGE_UP", "SCROLL_CURSOR_HOME", "SCROLL_CURSOR_END", "SCROLL_CURSOR_UP", "SCROLL_FILTER", "SCROLL_CANCEL", "SCROLL_SELECT_CURSOR_UP", "SCROLL_SELECT_CURSOR_DOWN", "SCROLL_SELECT_PAGE_DOWN", "SCROLL_SELECT_PAGE_UP", "SCROLL_SELECT_HOME", "SCROLL_SELECT_END", "SCROLL_SELECT", "SCROLL_SELECT_ALL", "SLE_LEFT", "SLE_RIGHT", "SLE_LEFT_WORD", "SLE_RIGHT_WORD", "SLE_LINE_START", "SLE_LINE_END", "SLE_DOC_START", "SLE_DOC_END", "SLE_DELETE_RIGHT", "SLE_DELETE_WORD_RIGHT", "SLE_DELETE_RIGHT_ALL", "SLE_DELETE_LEFT", "SLE_DELETE_WORD_LEFT", "SLE_DELETE_LEFT_ALL", "SLE_CUT", "SLE_COPY", "SLE_PASTE", "SLE_CYCLE_CASES", "SLE_SELECT_LEFT", "SLE_SELECT_RIGHT", "SLE_SELECT_LEFT_WORD", "SLE_SELECT_RIGHT_WORD", "SLE_SELECT_LINE_START", "SLE_SELECT_DOC_START", "SLE_SELECT_LINE_END", "SLE_SELECT_DOC_END", "SLE_SELECT_ALL", "SLE_UNDO", "SLE_REDO", "SLE_ACCEPT_FILTER", "SLE_AUTOCOMPLETE", "SLE_QUIT", "LAST_ACTION"}

func ActionStr(s Action) string {
	return ACTION_STR[s]
}

type Input struct {
	Keys     map[int32]bool
	MousePos rl.Vector2
	Blocked  bool
}

func StartBlockInput(input *Input) {
	input.Blocked = true
}
func EndBlockInput(input *Input) {
	if input.Blocked {
		input.Blocked = false
	}
}

func NewInput() *Input {
	return &Input{
		Keys: make(map[int32]bool, 0),
	}
}

const (
	KEY_MASK  int32 = (1 << 27) - 1
	MOD_SUPER int32 = 1 << 27
	MOD_SHIFT int32 = 1 << 28
	MOD_CTRL  int32 = 1 << 29
	MOD_ALT   int32 = 1 << 30
)

func DecodeKeyCombo(keyCombo int32) (ctrl, shift, alt, super bool, key int32) {
	return keyCombo&MOD_SHIFT == MOD_SHIFT,
		keyCombo&MOD_CTRL == MOD_CTRL,
		keyCombo&MOD_ALT == MOD_ALT,
		keyCombo&MOD_SUPER == MOD_SUPER,
		keyCombo & KEY_MASK
}

func KeyComboToStr(keycombo int32) string {
	ctrl, shift, alt, super, key := DecodeKeyCombo(keycombo)
	mod := ctrl || shift || alt || super
	var sb strings.Builder
	if ctrl {
		sb.WriteString("Ctrl")
	}
	if shift {
		sb.WriteString("Shift")
	}
	if alt {
		sb.WriteString("Alt")
	}
	if super {
		sb.WriteString("Super")
	}
	if mod {
		sb.WriteString(" + ")
	}
	sb.WriteString(KEYS_STR[key])
	return sb.String()
}

func UpdateInput(in *Input) {
	in.MousePos = rl.GetMousePosition()
	mods := int32(0)
	if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
		mods |= MOD_SHIFT
	}
	if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
		mods |= MOD_CTRL
	}
	if rl.IsKeyDown(rl.KeyLeftAlt) || rl.IsKeyDown(rl.KeyRightAlt) {
		mods |= MOD_ALT
	}
	if rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper) {
		mods |= MOD_SUPER
	}
	for _, key := range KEYS {
		mapKey := mods | key
		in.Keys[mapKey] = IsKeyPressedOrHeld(key)
	}

}
func (in *Input) CheckAndConsume(keyMask int32) bool {
	if in.Blocked {
		return false
	}
	if in.Keys[keyMask] {
		in.Keys[keyMask] = false
		return true
	}
	return false
}

func (in *Input) CheckAction(keybinds []KeyBind, consume bool) Action {
	inp := NONE
	for _, bind := range keybinds {
		if consume {
			if in.Keys[bind.KeyCombo] {
				in.Keys[bind.KeyCombo] = false
				inp = bind.Action
				break
			}
		} else {
			if in.Keys[bind.KeyCombo] {
				inp = bind.Action
				break
			}
		}
	}
	return inp
}

func IsKeyPressedOrHeld(key int32) bool {
	return rl.IsKeyPressed(key) || rl.IsKeyPressedRepeat(key)
}

type KeyBind struct {
	Action   Action
	KeyCombo int32
}

var KEYS = []int32{
	rl.KeySpace,
	rl.KeyEscape,
	rl.KeyEnter,
	rl.KeyTab,
	rl.KeyBackspace,
	rl.KeyInsert,
	rl.KeyDelete,
	rl.KeyRight,
	rl.KeyLeft,
	rl.KeyDown,
	rl.KeyUp,
	rl.KeyPageUp,
	rl.KeyPageDown,
	rl.KeyHome,
	rl.KeyEnd,
	rl.KeyCapsLock,
	rl.KeyScrollLock,
	rl.KeyNumLock,
	rl.KeyPrintScreen,
	rl.KeyPause,
	rl.KeyF1,
	rl.KeyF2,
	rl.KeyF3,
	rl.KeyF4,
	rl.KeyF5,
	rl.KeyF6,
	rl.KeyF7,
	rl.KeyF8,
	rl.KeyF9,
	rl.KeyF10,
	rl.KeyF11,
	rl.KeyF12,
	rl.KeyLeftShift,
	rl.KeyLeftControl,
	rl.KeyLeftAlt,
	rl.KeyLeftSuper,
	rl.KeyRightShift,
	rl.KeyRightControl,
	rl.KeyRightAlt,
	rl.KeyRightSuper,
	rl.KeyKbMenu,
	rl.KeyLeftBracket,
	rl.KeyBackSlash,
	rl.KeyRightBracket,
	rl.KeyGrave,
	rl.KeyKp0,
	rl.KeyKp1,
	rl.KeyKp2,
	rl.KeyKp3,
	rl.KeyKp4,
	rl.KeyKp5,
	rl.KeyKp6,
	rl.KeyKp7,
	rl.KeyKp8,
	rl.KeyKp9,
	rl.KeyKpDecimal,
	rl.KeyKpDivide,
	rl.KeyKpMultiply,
	rl.KeyKpSubtract,
	rl.KeyKpAdd,
	rl.KeyKpEnter,
	rl.KeyKpEqual,
	rl.KeyApostrophe,
	rl.KeyComma,
	rl.KeyMinus,
	rl.KeyPeriod,
	rl.KeySlash,
	rl.KeyZero,
	rl.KeyOne,
	rl.KeyTwo,
	rl.KeyThree,
	rl.KeyFour,
	rl.KeyFive,
	rl.KeySix,
	rl.KeySeven,
	rl.KeyEight,
	rl.KeyNine,
	rl.KeySemicolon,
	rl.KeyEqual,
	rl.KeyA,
	rl.KeyB,
	rl.KeyC,
	rl.KeyD,
	rl.KeyE,
	rl.KeyF,
	rl.KeyG,
	rl.KeyH,
	rl.KeyI,
	rl.KeyJ,
	rl.KeyK,
	rl.KeyL,
	rl.KeyM,
	rl.KeyN,
	rl.KeyO,
	rl.KeyP,
	rl.KeyQ,
	rl.KeyR,
	rl.KeyS,
	rl.KeyT,
	rl.KeyU,
	rl.KeyV,
	rl.KeyW,
	rl.KeyX,
	rl.KeyY,
	rl.KeyZ,
	rl.KeyBack,
	rl.KeyMenu,
	rl.KeyVolumeUp,
	rl.KeyVolumeDown,
}

var KEYS_STR = []string{
	rl.KeySpace:        "Space",
	rl.KeyEscape:       "Escape",
	rl.KeyEnter:        "Enter",
	rl.KeyTab:          "Tab",
	rl.KeyBackspace:    "Backspace",
	rl.KeyInsert:       "Insert",
	rl.KeyDelete:       "Delete",
	rl.KeyRight:        "Right",
	rl.KeyLeft:         "Left",
	rl.KeyDown:         "Down",
	rl.KeyUp:           "Up",
	rl.KeyPageUp:       "PageUp",
	rl.KeyPageDown:     "PageDown",
	rl.KeyHome:         "Home",
	rl.KeyEnd:          "End",
	rl.KeyCapsLock:     "CapsLock",
	rl.KeyScrollLock:   "ScrollLock",
	rl.KeyNumLock:      "NumLock",
	rl.KeyPrintScreen:  "PrintScreen",
	rl.KeyPause:        "Pause",
	rl.KeyF1:           "F1",
	rl.KeyF2:           "F2",
	rl.KeyF3:           "F3",
	rl.KeyF4:           "F4",
	rl.KeyF5:           "F5",
	rl.KeyF6:           "F6",
	rl.KeyF7:           "F7",
	rl.KeyF8:           "F8",
	rl.KeyF9:           "F9",
	rl.KeyF10:          "F10",
	rl.KeyF11:          "F11",
	rl.KeyF12:          "F12",
	rl.KeyLeftShift:    "LeftShift",
	rl.KeyLeftControl:  "LeftControl",
	rl.KeyLeftAlt:      "LeftAlt",
	rl.KeyLeftSuper:    "LeftSuper",
	rl.KeyRightShift:   "RightShift",
	rl.KeyRightControl: "RightControl",
	rl.KeyRightAlt:     "RightAlt",
	rl.KeyRightSuper:   "RightSuper",
	rl.KeyKbMenu:       "KbMenu",
	rl.KeyLeftBracket:  "LeftBracket",
	rl.KeyBackSlash:    "BackSlash",
	rl.KeyRightBracket: "RightBracket",
	rl.KeyGrave:        "Grave",
	rl.KeyKp0:          "Kp0",
	rl.KeyKp1:          "Kp1",
	rl.KeyKp2:          "Kp2",
	rl.KeyKp3:          "Kp3",
	rl.KeyKp4:          "Kp4",
	rl.KeyKp5:          "Kp5",
	rl.KeyKp6:          "Kp6",
	rl.KeyKp7:          "Kp7",
	rl.KeyKp8:          "Kp8",
	rl.KeyKp9:          "Kp9",
	rl.KeyKpDecimal:    "KpDecimal",
	rl.KeyKpDivide:     "KpDivide",
	rl.KeyKpMultiply:   "KpMultiply",
	rl.KeyKpSubtract:   "KpSubtract",
	rl.KeyKpAdd:        "KpAdd",
	rl.KeyKpEnter:      "KpEnter",
	rl.KeyKpEqual:      "KpEqual",
	rl.KeyApostrophe:   "Apostrophe",
	rl.KeyComma:        "Comma",
	rl.KeyMinus:        "Minus",
	rl.KeyPeriod:       "Period",
	rl.KeySlash:        "Slash",
	rl.KeyZero:         "Zero",
	rl.KeyOne:          "One",
	rl.KeyTwo:          "Two",
	rl.KeyThree:        "Three",
	rl.KeyFour:         "Four",
	rl.KeyFive:         "Five",
	rl.KeySix:          "Six",
	rl.KeySeven:        "Seven",
	rl.KeyEight:        "Eight",
	rl.KeyNine:         "Nine",
	rl.KeySemicolon:    "Semicolon",
	rl.KeyEqual:        "Equal",
	rl.KeyA:            "A",
	rl.KeyB:            "B",
	rl.KeyC:            "C",
	rl.KeyD:            "D",
	rl.KeyE:            "E",
	rl.KeyF:            "F",
	rl.KeyG:            "G",
	rl.KeyH:            "H",
	rl.KeyI:            "I",
	rl.KeyJ:            "J",
	rl.KeyK:            "K",
	rl.KeyL:            "L",
	rl.KeyM:            "M",
	rl.KeyN:            "N",
	rl.KeyO:            "O",
	rl.KeyP:            "P",
	rl.KeyQ:            "Q",
	rl.KeyR:            "R",
	rl.KeyS:            "S",
	rl.KeyT:            "T",
	rl.KeyU:            "U",
	rl.KeyV:            "V",
	rl.KeyW:            "W",
	rl.KeyX:            "X",
	rl.KeyY:            "Y",
	rl.KeyZ:            "Z",
	rl.KeyBack:         "Back",
	rl.KeyMenu:         "Menu",
	rl.KeyVolumeUp:     "VolumeUp",
	rl.KeyVolumeDown:   "VolumeDown",
}

type SLEState struct {
	Cursor    int
	Text      []rune
	Selection int
}

type SingleLineEditor struct {
	SLEState
	OnClipboardCopy  func(in string)
	OnClipboardPaste func() string
	WordBreaks       []rune
	EnclosersOpen    []rune
	EnclosersClose   []rune
	BlinkTimer       float32
	ShowCursor       bool

	History[SLEState]
}

type History[T any] struct {
	State []T
	Index int
}

func (s *History[T]) Append(n T) {
	if s.State == nil {
		s.State = make([]T, 0)
	}
	if len(s.State) == 0 || s.Index == len(s.State)-1 {
		s.State = append(s.State, n)
		s.Index += 1
		return
	}
	s.State = append(s.State[:s.Index], n)
	s.Index += 1
}
func (s *History[T]) Redo() bool {
	if s.Index+1 < len(s.State) {
		s.Index += 1
		return true
	}
	return false
}
func (s *History[T]) Undo() bool {
	if s.Index-1 >= 0 {
		s.Index -= 1
		return true
	}
	return false
}
func (s *History[T]) Current() T {
	return s.State[s.Index]
}

func NewSingleLineEditor4(
	text []rune,
	onClipboardCopy func(in string),
	onClipboardPaste func() string,
	wordBreaks []rune,
	enclosersOpen []rune,
	enclosersClose []rune,
) *SingleLineEditor {
	sle := &SingleLineEditor{
		SLEState: SLEState{
			Cursor:    0,
			Text:      make([]rune, len(text)),
			Selection: 0,
		},
		OnClipboardCopy:  onClipboardCopy,
		OnClipboardPaste: onClipboardPaste,
		WordBreaks:       wordBreaks,
		EnclosersOpen:    enclosersOpen,
		EnclosersClose:   enclosersClose,
	}
	sle.Text = text
	sle.Cursor = len(text)
	return sle
}
func NewSingleLineEditor3(
	text []rune,
	onClipboardCopy func(in string),
	onClipboardPaste func() string,
) *SingleLineEditor {
	sle := NewSingleLineEditor4(
		text,
		onClipboardCopy,
		onClipboardPaste,
		WORD_BREAKS,
		ENCLOSERS_OPEN,
		ENCLOSERS_CLOSE,
	)
	return sle
}
func NewSingleLineEditor2(text []rune) *SingleLineEditor {
	return NewSingleLineEditor4(
		text,
		rl.SetClipboardText,
		rl.GetClipboardText,
		WORD_BREAKS,
		ENCLOSERS_OPEN,
		ENCLOSERS_CLOSE,
	)
}
func NewSingleLineEditor() *SingleLineEditor {
	return NewSingleLineEditor4(
		[]rune{},
		rl.SetClipboardText,
		rl.GetClipboardText,
		WORD_BREAKS,
		ENCLOSERS_OPEN,
		ENCLOSERS_CLOSE,
	)
}

var WORD_BREAKS = []rune(" ~!@#$%^&*()-=+[{]}\\|;:'\",.<>?/`")

var ENCLOSERS_OPEN = []rune("{[\"'`(")
var ENCLOSERS_CLOSE = []rune("}]\"'`)")

func (s *SingleLineEditor) isWordBreak(r rune) bool {
	for _, w := range s.WordBreaks {
		if r == w {
			return true
		}
	}
	return false
}

func (s *SingleLineEditor) isEncloser(r rune) (rune, bool) {
	for i, w := range s.EnclosersOpen {
		if r == w {
			return s.EnclosersClose[i], true
		}
	}
	return ' ', false
}

var DEFAULT_SLE_KEYBINDINGS = []KeyBind{
	{SLE_DELETE_LEFT_ALL, MOD_CTRL | MOD_SHIFT | rl.KeyBackspace},
	{SLE_DELETE_WORD_LEFT, MOD_CTRL | rl.KeyBackspace},
	{SLE_DELETE_LEFT, rl.KeyBackspace},
	{SLE_DELETE_RIGHT_ALL, MOD_CTRL | MOD_SHIFT | rl.KeyDelete},
	{SLE_DELETE_WORD_RIGHT, MOD_CTRL | rl.KeyDelete},
	{SLE_DELETE_RIGHT, rl.KeyDelete},
	{SLE_COPY, MOD_CTRL | rl.KeyC},
	{SLE_CUT, MOD_CTRL | rl.KeyX},
	{SLE_PASTE, MOD_CTRL | rl.KeyV},
	{SLE_SELECT_ALL, MOD_CTRL | rl.KeyA},
	{SLE_SELECT_LEFT_WORD, MOD_CTRL | MOD_SHIFT | rl.KeyLeft},
	{SLE_SELECT_LEFT, MOD_SHIFT | rl.KeyLeft},
	{SLE_LEFT_WORD, MOD_CTRL | rl.KeyLeft},
	{SLE_LEFT, rl.KeyLeft},
	{SLE_SELECT_LINE_START, MOD_CTRL | MOD_SHIFT | rl.KeyHome},
	{SLE_SELECT_DOC_START, MOD_SHIFT | rl.KeyHome},
	{SLE_DOC_START, MOD_CTRL | rl.KeyHome},
	{SLE_LINE_START, rl.KeyHome},
	{SLE_SELECT_RIGHT_WORD, MOD_CTRL | MOD_SHIFT | rl.KeyRight},
	{SLE_SELECT_RIGHT, MOD_SHIFT | rl.KeyRight},
	{SLE_RIGHT_WORD, MOD_CTRL | rl.KeyRight},
	{SLE_RIGHT, rl.KeyRight},
	{SLE_SELECT_LINE_END, MOD_CTRL | MOD_SHIFT | rl.KeyEnd},
	{SLE_SELECT_DOC_END, MOD_SHIFT | rl.KeyEnd},
	{SLE_DOC_END, MOD_CTRL | rl.KeyEnd},
	{SLE_LINE_END, rl.KeyEnd},
	{SLE_UNDO, MOD_CTRL | rl.KeyZ},
	{SLE_REDO, MOD_CTRL | rl.KeyY},
	{SLE_QUIT, rl.KeyEscape},
	{SLE_ACCEPT_FILTER, rl.KeyEnter},
	{SLE_AUTOCOMPLETE, rl.KeyTab},
}

func GetTextInput() []rune {
	textInput := make([]rune, 0)
	key := rl.GetCharPressed()
	for key > 0 {
		if key >= 32 && key <= 125 {
			textInput = append(textInput, rune(key))
		}
		key = rl.GetCharPressed()
	}
	return textInput
}

type SLEResult int

const (
	SLE_R_NO_INPUT SLEResult = iota
	SLE_R_HAD_INPUT
	SLE_R_HAD_CHAR_INPUT
	SLE_R_AUTOCOMPLETE
	SLE_R_ACCEPT
	SLE_R_QUIT
)

func (s *SingleLineEditor) Update(action Action, textInput []rune) SLEResult {
	if len(textInput) > 0 {
		twin, ok := s.isEncloser(textInput[0])
		if s.Selection != 0 && len(textInput) == 1 && ok {
			if s.Selection < 0 {
				temp := slices.Clone(s.Text[s.Cursor : s.Cursor-s.Selection])
				temp = slices.Insert(temp, 0, textInput...)
				temp = append(temp, []rune{twin}...)
				s.Text = slices.Delete(s.Text, s.Cursor, s.Cursor-s.Selection)
				s.Text = slices.Insert(s.Text, s.Cursor, temp...)
				s.Selection = len(temp) - 2
				s.Cursor += len(temp) - 1
				return SLE_R_HAD_CHAR_INPUT
			} else if s.Selection > 0 {
				temp := slices.Clone(s.Text[s.Cursor-s.Selection : s.Cursor])
				temp = slices.Insert(temp, 0, textInput...)
				temp = append(temp, []rune{twin}...)
				s.Text = slices.Delete(s.Text, s.Cursor-s.Selection, s.Cursor)
				s.Cursor = s.Cursor - s.Selection
				s.Text = slices.Insert(s.Text, s.Cursor, temp...)
				s.Selection = len(temp) - 2
				s.Cursor += len(temp) - 1
				return SLE_R_HAD_CHAR_INPUT
			}
		} else {
			if s.Selection < 0 {
				s.Text = slices.Delete(s.Text, s.Cursor, s.Cursor-s.Selection)
				s.Selection = 0
			} else if s.Selection > 0 {
				s.Text = slices.Delete(s.Text, s.Cursor-s.Selection, s.Cursor)
				s.Cursor = s.Cursor - s.Selection
				s.Selection = 0
			}
		}
		s.Text = slices.Insert(s.Text, s.Cursor, textInput...)
		//todo:check here if any word breaks were added, if yes, append to history
		s.Cursor += len(textInput)
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	}
	hadInput := false
	if action != NONE {
		hadInput = true
	}
	switch action {
	case SLE_LEFT:
		if s.Cursor > 0 {
			if s.Selection > 0 {
				s.Cursor -= s.Selection
			} else if s.Selection < 0 {
				//nothing
			} else {
				s.Cursor--
			}
		}
		s.Selection = 0
	case SLE_RIGHT:
		if s.Cursor < len(s.Text) {
			if s.Selection < 0 {
				s.Cursor += -s.Selection
			} else if s.Selection > 0 {
				//nothing
			} else {
				s.Cursor++
			}
		}
		s.Selection = 0
	case SLE_LEFT_WORD:
		if s.Cursor > 0 {
			for s.Cursor > 0 && s.isWordBreak((s.Text)[s.Cursor-1]) {
				s.Cursor--
			}
			for s.Cursor > 0 && !s.isWordBreak((s.Text)[s.Cursor-1]) {
				s.Cursor--
			}
		}
		s.Selection = 0
	case SLE_RIGHT_WORD:
		if s.Cursor < len(s.Text) {
			for s.Cursor < len(s.Text) && s.isWordBreak((s.Text)[s.Cursor]) {
				s.Cursor++
			}
			for s.Cursor < len(s.Text) && !s.isWordBreak((s.Text)[s.Cursor]) {
				s.Cursor++
			}
		}
		s.Selection = 0
	case SLE_LINE_START:
		s.Cursor = 0
		s.Selection = 0
	case SLE_LINE_END:
		s.Cursor = len(s.Text)
		s.Selection = 0
	case SLE_DOC_START:
		s.Cursor = 0
		s.Selection = 0
	case SLE_DOC_END:
		s.Cursor = len(s.Text)
		s.Selection = 0
	case SLE_SELECT_LEFT:
		prevPos := s.Cursor
		if s.Cursor > 0 {
			if s.Selection > 0 {
				s.Cursor -= s.Selection
			} else {
				s.Cursor--
			}
		}
		if s.Selection == 0 {
			s.Selection = s.Cursor - prevPos
		} else {
			s.Selection -= prevPos - s.Cursor
		}
	case SLE_SELECT_RIGHT:
		prevPos := s.Cursor
		if s.Cursor < len(s.Text) {
			if s.Selection < 0 {
				s.Cursor += -s.Selection
			} else {
				s.Cursor++
			}
		}
		if s.Selection == 0 {
			s.Selection = s.Cursor - prevPos
		} else {
			s.Selection += s.Cursor - prevPos
		}
	case SLE_SELECT_LEFT_WORD:
		prevPos := s.Cursor
		if s.Cursor > 0 {
			for s.Cursor > 0 && s.isWordBreak((s.Text)[s.Cursor-1]) {
				s.Cursor--
			}
			for s.Cursor > 0 && !s.isWordBreak((s.Text)[s.Cursor-1]) {
				s.Cursor--
			}
		}
		if s.Selection == 0 {
			s.Selection = s.Cursor - prevPos
		} else {
			s.Selection -= prevPos - s.Cursor
		}
	case SLE_SELECT_RIGHT_WORD:
		prevPos := s.Cursor
		if s.Cursor < len(s.Text) {
			for s.Cursor < len(s.Text) && s.isWordBreak((s.Text)[s.Cursor]) {
				s.Cursor++
			}
			for s.Cursor < len(s.Text) && !s.isWordBreak((s.Text)[s.Cursor]) {
				s.Cursor++
			}
		}
		if s.Selection == 0 {
			s.Selection = s.Cursor - prevPos
		} else {
			s.Selection += s.Cursor - prevPos
		}
	case SLE_SELECT_LINE_START, SLE_SELECT_DOC_START:
		prevPos := s.Cursor
		s.Cursor = 0
		if s.Selection == 0 {
			s.Selection = s.Cursor - prevPos
		} else {
			s.Selection -= prevPos - s.Cursor
		}

	case SLE_SELECT_LINE_END, SLE_SELECT_DOC_END:
		prevPos := s.Cursor
		s.Cursor = len(s.Text)
		if s.Selection == 0 {
			s.Selection = s.Cursor - prevPos
		} else {
			s.Selection += s.Cursor - prevPos
		}
	case SLE_SELECT_ALL:
		s.Selection = len(s.Text)
		s.Cursor = len(s.Text)
	case SLE_DELETE_RIGHT:
		if s.Selection == 0 {
			if s.Cursor < len(s.Text) {
				s.Text = slices.Delete(s.Text, s.Cursor, s.Cursor+1)
			}
		} else {
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_DELETE_WORD_RIGHT:
		if s.Selection == 0 {
			prevPos := s.Cursor
			if s.Cursor < len(s.Text) {
				for s.Cursor < len(s.Text) && s.isWordBreak(s.Text[s.Cursor]) {
					s.Cursor++
				}
				for s.Cursor < len(s.Text) && !s.isWordBreak(s.Text[s.Cursor]) {
					s.Cursor++
				}
			}
			s.Text = slices.Delete(s.Text, prevPos, s.Cursor)
			s.Cursor = prevPos
		} else {
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_DELETE_RIGHT_ALL:
		if s.Selection == 0 {
			if s.Cursor < len(s.Text) {
				s.Text = slices.Delete(s.Text, s.Cursor, len(s.Text))
				s.Cursor = len(s.Text)
			}
		} else {
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_DELETE_LEFT:
		if s.Selection == 0 {
			if s.Cursor > 0 {
				s.Text = slices.Delete(s.Text, s.Cursor-1, s.Cursor)
				s.Cursor--
			}
		} else {
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_DELETE_WORD_LEFT:
		if s.Selection == 0 {
			prevPos := s.Cursor
			if s.Cursor > 0 {
				for s.Cursor > 0 && s.isWordBreak((s.Text)[s.Cursor-1]) {
					s.Cursor--
				}
				for s.Cursor > 0 && !s.isWordBreak((s.Text)[s.Cursor-1]) {
					s.Cursor--
				}
			}
			s.Text = slices.Delete(s.Text, s.Cursor, prevPos)
		} else {
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_DELETE_LEFT_ALL:
		if s.Selection == 0 {
			if s.Cursor > 0 {
				s.Text = slices.Delete(s.Text, 0, s.Cursor)
				s.Cursor = 0
			}
		} else {
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_CUT:
		if s.Selection == 0 {
			s.OnClipboardCopy(string(s.Text))
			s.Text = make([]rune, 0)
			s.Selection = 0
			s.Cursor = 0
		} else {
			if s.Selection < 0 {
				s.OnClipboardCopy(string((s.Text)[s.Cursor : s.Cursor-s.Selection]))
			} else if s.Selection > 0 {
				s.OnClipboardCopy(string((s.Text)[s.Cursor-s.Selection : s.Cursor]))
			}
			s.resetSelection()
		}
		s.History.Append(s.SLEState)
		return SLE_R_HAD_CHAR_INPUT
	case SLE_COPY:
		if s.Selection == 0 {
			s.OnClipboardCopy(string(s.Text))
		} else if s.Selection < 0 {
			s.OnClipboardCopy(string((s.Text)[s.Cursor : s.Cursor-s.Selection]))
		} else if s.Selection > 0 {
			s.OnClipboardCopy(string((s.Text)[s.Cursor-s.Selection : s.Cursor]))
		}
	case SLE_PASTE:
		s.resetSelection()
		text := []rune(strings.ReplaceAll(s.OnClipboardPaste(), "\n", " "))
		s.Text = slices.Insert(s.Text, s.Cursor, text...)
		s.Cursor += len(text)
		s.History.Append(s.SLEState)
	case SLE_CYCLE_CASES:
		fmt.Printf("Unimplemented: cycling through uppercase/lowercase/snake case/etc\n")
	case SLE_UNDO:
		if s.History.Undo() {
			s.SLEState = s.History.Current()
		}
	case SLE_REDO:
		if s.History.Redo() {
			s.SLEState = s.History.Current()
		}
	case SLE_ACCEPT_FILTER:
		return SLE_R_ACCEPT
	case SLE_QUIT:
		return SLE_R_QUIT
	case SLE_AUTOCOMPLETE:
		return SLE_R_AUTOCOMPLETE

	case NONE:
		return SLE_R_NO_INPUT
	default:
		println("SLE: action without implementation: %d", action)
	}
	if hadInput {
		return SLE_R_HAD_INPUT
	}
	return SLE_R_NO_INPUT
}

func (s *SingleLineEditor) resetSelection() {
	if s.Selection < 0 {
		s.Text = slices.Delete(s.Text, s.Cursor, s.Cursor-s.Selection)
	} else {
		s.Text = slices.Delete(s.Text, s.Cursor-s.Selection, s.Cursor)
		s.Cursor = s.Cursor - s.Selection
	}
	s.Selection = 0
}

func (s *SingleLineEditor) Input(input *Input, keyBinds []KeyBind) SLEResult {
	textInput := GetTextInput()
	action := input.CheckAction(keyBinds, true)
	result := s.Update(action, textInput)
	if action != NONE {
		s.BlinkTimer = 1
	}
	return result
}

type SingleLineEditorTheme struct {
	Text      rl.Color
	Cursor    rl.Color
	Selection rl.Color
}

func NewDefaultSingleLineEditorTheme() SingleLineEditorTheme {
	return SingleLineEditorTheme{rl.White, rl.White, rl.Blue}
}

func DrawSLE(
	font Font,
	s *SingleLineEditor,
	pos rl.Vector2,
	showCursor bool,
	dt float32,
) {
	theme := NewDefaultSingleLineEditorTheme()
	DrawSLEEx(font, s, pos, theme, showCursor, dt)
}

func DrawSLEEx(
	font Font,
	s *SingleLineEditor,
	pos rl.Vector2,
	theme SingleLineEditorTheme,
	showCursor bool,
	dt float32,
) {
	if s == nil {
		return
	}
	if s.Selection != 0 {
		ini := s.Cursor
		end := s.Cursor - s.Selection
		if s.Selection > 0 {
			ini = s.Cursor - s.Selection
			end = s.Cursor
		}
		selectionOffset := MeasureText(font, string(s.Text[:ini])).X
		selectionWidth := MeasureText(font, string(s.Text[ini:end])).X
		selectionRec := rl.Rectangle{
			pos.X + selectionOffset, pos.Y,
			selectionWidth, font.Size,
		}
		rl.DrawRectangleRec(selectionRec, theme.Selection)
	}
	DrawText(font, pos, string(s.Text), theme.Text)
	cursorPosX := MeasureText(font, string(s.Text[:s.Cursor])).X
	rec := rl.Rectangle{pos.X + cursorPosX, pos.Y, 2, font.Size}
	s.BlinkTimer -= dt
	if s.BlinkTimer > 0.5 && showCursor {
		rl.DrawRectangleRec(rec, theme.Cursor)
	}
	if s.BlinkTimer < 0 {
		s.BlinkTimer = 1
	}
}

func (s *SingleLineEditor) Reset() {
	s.Cursor = 0
	s.Selection = 0
	s.Text = make([]rune, 0)
}

func NotShitMod[T ~int](a, b T) T {
	if b == 0 {
		return 0
	}
	return (a%b + b) % b
}
func Clamp[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](val, min, max T) T {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
func ClampMin[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](val, min T) T {
	if val < min {
		return min
	}
	return val
}
func ClampMax[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](val, max T) T {
	if val > max {
		return max
	}
	return val
}
func Min[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](a, b T) T {
	if a < b {
		return a
	}
	return b
}
func Max[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}
func Abs[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](x T) T {
	return T(math.Abs(float64(x)))
}
func Ceil[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](x T) T {
	return T(math.Ceil(float64(x)))
}
func Floor[T int | uint | uint32 | uint64 | int32 | int64 | float32 | float64](x T) T {
	return T(math.Floor(float64(x)))
}

func RectangleEquals(a, b rl.Rectangle) bool {
	return a.X == b.X && a.Y == b.Y && a.Width == b.Width && a.Height == b.Height
}

func RecToVec(r rl.Rectangle) (rl.Vector2, rl.Vector2) {
	return rl.Vector2{r.X, r.Y}, rl.Vector2{r.Width, r.Height}
}
func GetRecCenter(r rl.Rectangle) rl.Vector2 {
	return rl.Vector2{r.X + r.Width/2, r.Y + r.Height/2}
}

func ExpandHome(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////
//test suite
////////////////////////////////////////////////////////////////////////////////////////////////////
