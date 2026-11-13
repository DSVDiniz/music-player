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
	_ "embed"
	"encoding/gob"
	"flag"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	tag "github.com/dhowden/tag"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func ListenHotkeys(c chan Action) {
	X, err := xgb.NewConn()
	if err != nil {
		LOG.Panic("%s", err.Error())
	}

	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)
	root := screen.Root

	keys := []uint32{
		197, //??
		196, //??
		195, //XF86Launch9
		194, //XF86Launch8
		193, //XF86Launch6
		192, //XF86Launch5
		191, //XF86Tools
	}
	modsList := []uint16{
		0,
		xproto.ModMaskLock,
		xproto.ModMask2,
		xproto.ModMask2 | xproto.ModMaskLock,
	}
	for _, keycode := range keys {
		for _, mods := range modsList {
			err = xproto.GrabKeyChecked(
				X,
				true,
				root,
				mods,
				xproto.Keycode(keycode),
				xproto.GrabModeAsync,
				xproto.GrabModeAsync,
			).Check()
			if err != nil {
				LOG.Warn("failed to grab keycode %d: %v", keycode, err)
			}
		}

	}

	for {
		ev, err := X.WaitForEvent()
		if err != nil {
			LOG.Warn("%s", err.Error())
			continue
		}

		switch e := ev.(type) {
		case xproto.KeyPressEvent:
			switch e.Detail {
			case 195: //XF86Launch9
				c <- CHANGE_PLAYORDER_RANDOM
			case 197: //??
				c <- CHANGE_PLAYORDER_RANDOM_NO_REPEAT
			case 194: //XF86Launch8
				c <- CHANGE_PLAYORDER_NORMAL
			case 191: //XF86Tools
				c <- PLAY_PAUSE // play/pause
			case 192: //XF86Launch5
				c <- NEXT
			case 193: //XF86Launch6
				c <- PREVIOUS
			case 196: //??
				c <- NEXT_RANDOM
				// case 196: //??
				// 	c <- VOLUME_DOWN
				// case 196: //??
				// 	c <- VOLUME_UP
			}
		}
	}
}

var FONT Font

var line_height float32

var M_THEME = MusicPlayerTheme{
	Background: rl.Black,
	Default:    BoxTextTheme{1, rl.White, rl.White, rl.Black},
	Debug:      BoxTextTheme{1, rl.White, rl.White, rl.Black},
	Title: TitleTheme{
		Thickness: 1,
		Text:      rl.White,
		Border:    rl.White,
		Bg1:       rl.Black,
		Bg2:       rl.Blue,
	},

	Volume: ProgressBarTheme{
		Base:   BoxTheme{1, rl.White, rl.Black},
		Bar:    BoxTheme{1, rl.White, rl.DarkGreen},
		Text:   rl.White,
		TextBg: rl.Black,
	},
	Time: ProgressBarTheme{
		Base:   BoxTheme{1, rl.White, rl.Black},
		Bar:    BoxTheme{1, rl.White, DarkYellow},
		Text:   rl.White,
		TextBg: rl.Black,
	},
	PlayMode: PlayModeBarTheme{
		Base:   BoxTheme{1, rl.White, rl.Black},
		Text:   rl.White,
		TextBg: rl.Black,
	},
	CurrentSong: CurrentSongBarTheme{
		Base:   BoxTheme{1, rl.White, rl.Black},
		Text:   rl.White,
		TextBg: rl.Black,
	},
	CurrentSongInfo: BoxTextTheme{1, rl.White, rl.White, rl.Black},

	FileColors: map[string]FileTypeColor{
		".mp3":  {rl.White, rl.Black, rl.Black, rl.White},
		".wav":  {rl.DarkBlue, rl.Black, rl.Black, rl.DarkBlue},
		".flac": {rl.Purple, rl.Black, rl.White, rl.Purple},
		".vgz":  {rl.DarkGreen, rl.Black, rl.White, rl.DarkGreen},
		".ogg":  {DarkRed, rl.Black, rl.White, DarkRed},
		".webm": {rl.DarkGray, rl.Black, rl.White, rl.DarkGray},
		".mp4":  {rl.Brown, rl.Black, rl.White, rl.Brown},
		".spc":  {rl.Beige, rl.Black, rl.White, rl.Beige},
	},
	SelectedFileText:    rl.Black,
	SelectedFileBg:      rl.Yellow,
	HighlightedFileText: rl.Black,
	HighlightedFileBg:   rl.Gold,
	PlayingText:         rl.White,
	PlayingBg:           rl.Red,

	ScrollHeader: BoxTextTheme{1, rl.White, rl.White, rl.Gray},

	Scroll: NewDefaultScrollTheme(),
}

type MusicPlayerTheme struct {
	Background rl.Color
	Default    BoxTextTheme
	Title      TitleTheme
	Debug      BoxTextTheme

	Volume          ProgressBarTheme
	Time            ProgressBarTheme
	PlayMode        PlayModeBarTheme
	CurrentSong     CurrentSongBarTheme
	CurrentSongInfo BoxTextTheme

	FileColors          map[string]FileTypeColor
	SelectedFileText    rl.Color
	SelectedFileBg      rl.Color
	HighlightedFileText rl.Color
	HighlightedFileBg   rl.Color
	PlayingText         rl.Color
	PlayingBg           rl.Color

	ScrollHeader BoxTextTheme

	Scroll ScrollTheme
}
type TitleTheme struct {
	Thickness float32
	Text      rl.Color
	Border    rl.Color
	Bg1       rl.Color
	Bg2       rl.Color
}
type ProgressBarTheme struct {
	Base   BoxTheme
	Bar    BoxTheme
	Text   rl.Color
	TextBg rl.Color
}
type PlayModeBarTheme struct {
	Base   BoxTheme
	Text   rl.Color
	TextBg rl.Color
}
type CurrentSongBarTheme struct {
	Base   BoxTheme
	Text   rl.Color
	TextBg rl.Color
}
type FileTypeColor struct {
	Text            rl.Color
	Bg              rl.Color
	HighlightedText rl.Color
	HighlightedBg   rl.Color
}

type ScreenData struct {
	HasResized      bool
	PrevFrameScreen rl.Vector2
}

var PLAYER MusicPlayer

type MusicPlayer struct {
	CurrentMusic *MusicFile
	RaylibMusic  *rl.Music
	//CustomMusic  CustomPCMSource
	State      MusicState
	Volume     float32
	TimePlayed float32
	TimeLength float32

	PlaylistPath    string
	HeaderCols      []HeaderCol
	Playlist        []*MusicFile
	SelectedSongs   map[int]struct{}
	PlayedSongs     []int
	PlayOrder       PlayOrder
	RemainingUnique []int
	Playcount       map[string]int
}

//go:embed thunder-rumble-short.mp3
var soundData []byte

const state_filename = "playcount_data"
const config_folder = "~/.config/music-player/"
const log_filename = "~/.config/music-player/log.log"

func main() {
	defer func() {
		if r := recover(); r != nil {
			LOG.Error("PANIC: %v\n%s", r, debug.Stack())
			LOG.Info("%s", PLAYER.CurrentMusic.RelativePath)
		}
	}()

	mainLoop()
}

var LOG *Logger

func mainLoop() {
	rand.Seed(time.Now().UnixNano())

	hotkeyChan := make(chan Action)

	go ListenHotkeys(hotkeyChan)

	var p string
	var noise bool
	flag.StringVar(&p, "path", "", "the folder where all unorganized music files are (recursive)")
	flag.BoolVar(&noise, "noise", true, "play cyclical sound")
	flag.Parse()
	var path string
	var err error
	if len(p) > 0 {
		path, err = ExpandHome(p)
		if err != nil {
			panic(err)
		}
	}
	logPath, err := ExpandHome(log_filename)
	if err != nil {
		panic(err)
	}
	LOG, err = NewLogger(logPath)
	if err != nil {
		panic(err)
	}

	gob.Register(MusicPlayerNonPointer{})
	gob.Register(MusicFile{})
	gob.Register(rl.Music{})
	gob.Register(HeaderCol{})

	state, err := LoadState()
	if err != nil {
		LOG.Info("%v : loading default player options\n", err.Error())
	}

	go MonitorMemory()
	rl.SetTraceLogLevel(rl.LogNone)
	rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(800, 600, "music-player")
	defer rl.CloseWindow()
	rl.SetExitKey(rl.KeyNull)
	rl.InitAudioDevice()

	SHADER = rl.LoadShader("shaders/shader.vs", "shaders/shader.fs")
	RESOLUTIONLOC = rl.GetShaderLocation(SHADER, "resolution")
	TIMELOC = rl.GetShaderLocation(SHADER, "time")
	COLORLOC = rl.GetShaderLocation(SHADER, "uColor")

	DIM_SHADER = rl.LoadShader("shaders/dim_shader.vs", "shaders/dim_shader.fs")
	DIM_FACTORLOC = rl.GetShaderLocation(DIM_SHADER, "dimFactor")
	//fontPath := "roboto.ttf"
	fontPath, err := GetSystemFont()
	if err != nil {
		LOG.Warn("couldn't load system font: %s", err)
	}
	const font_size = 20
	const codepointSize = 20000
	FONT = NewFont(fontPath, font_size, codepointSize)
	FONT.IsMonospace = true
	defer rl.UnloadFont(FONT.Font)

	scroll := Scroll[*MusicFile]{
		ItemHeight: FONT.Size,
		FilterSLE:  NewSingleLineEditor(),
	}

	if state == nil {
		LOG.Info("couldn't find saved state, using default values")
		PLAYER = MusicPlayer{
			Volume:        1,
			PlayedSongs:   make([]int, 0),
			SelectedSongs: make(map[int]struct{}, 0),
			Playcount:     make(map[string]int, 0),
			PlayOrder:     PLAYORDER_RANDOM_NO_REPEAT,
		}
		if len(path) == 0 {
			LOG.Panic("no path was specified and no saved data was found, use --help")
		}
		musicfiles, headerCols, err := LoadMusicFiles(path)
		if err != nil {
			LOG.Panic("%v", err.Error())
		}
		PLAYER.PlaylistPath = path
		PLAYER.HeaderCols = headerCols
		PLAYER.Playlist = musicfiles
	} else {
		LOG.Info("loading saved state")
		if len(path) == 0 && len(state.PlaylistPath) == 0 {
			LOG.Panic("no path was specified and saved data had no playlist path, use --help and at least add a path")
		}
		if !FileExists(state.PlaylistPath) {
			if len(path) == 0 {
				LOG.Panic("saved path does not exist and no path was specified!")
			}
		}
		if len(path) > 0 {
			LOG.Info("overriding previous saved path\n")
			state.PlaylistPath = path
		}
		PLAYER = MusicPlayer{
			RaylibMusic: nil,
			State:       state.State,
			Volume:      state.Volume,
			TimePlayed:  state.TimePlayed,
			TimeLength:  state.TimeLength,

			PlaylistPath: state.PlaylistPath,
			HeaderCols:   state.HeaderCols,
			//Playlist:        state.Playlist,
			PlayedSongs:   make([]int, 0),         //state.SelectedSongs,
			SelectedSongs: make(map[int]struct{}), //state.PlayedSongs,
			PlayOrder:     state.PlayOrder,
			//RemainingUnique: state.RemainingUnique,
			Playcount: state.Playcount,
		}
		musicfiles, headerCols, err := LoadMusicFiles(PLAYER.PlaylistPath)
		if err != nil {
			LOG.Panic("%v", err.Error())
		}
		var currentMusic *MusicFile
		if len(musicfiles) > 0 {
			for _, m := range musicfiles {
				if m.File.Path == state.CurrentMusicPath {
					currentMusic = m
					break
				}
			}
		}
		PLAYER.CurrentMusic = currentMusic
		//if len(PLAYER.HeaderCols) == 0 {
		PLAYER.HeaderCols = headerCols
		//}
		PLAYER.Playlist = musicfiles
		if PLAYER.CurrentMusic != nil {
			mus := rl.LoadMusicStream(PLAYER.CurrentMusic.File.Path)
			PLAYER.RaylibMusic = &mus
			//I need to do this song and dance, otherwise song isn't really loaded and paused
			rl.PlayMusicStream(*PLAYER.RaylibMusic)
			rl.SetMusicVolume(*PLAYER.RaylibMusic, PLAYER.Volume)
			rl.PauseMusicStream(*PLAYER.RaylibMusic)
			PLAYER.State = PAUSED
			for i, v := range PLAYER.Playlist {
				if v.File.Path == PLAYER.CurrentMusic.File.Path {
					scroll.Cursor = i
				}
			}
		}
	}

	playlistTitle := fmt.Sprintf("%s", PLAYER.PlaylistPath)
	title := fmt.Sprintf("music-player - %s", playlistTitle)

	input := NewInput()
	screenData := &ScreenData{}
	exit := rl.WindowShouldClose()
	showFps := false
	screen := rl.Rectangle{}

	line_height = FONT.LineSpacing + FONT.Size

	cycle := toDuration(170)
	rumbleSound := NewCyclicalSound(soundData, cycle)
	firstFrame := true
	for !exit {
		if noise {
			UpdateCycle(rumbleSound)
			if PLAYER.State == PLAYING && rumbleSound.State != CYC_STOP {
				rumbleSound.State = CYC_STOP
			} else if (PLAYER.State == STOPPED || PLAYER.State == PAUSED) && (rumbleSound.State == CYC_STOP) {
				rumbleSound.State = CYC_START
			}
		}

		globalHotkeyAction := NONE
		select {
		case act := <-hotkeyChan:
			globalHotkeyAction = act
		default:
		}

		dt := rl.GetFrameTime()
		exit = rl.WindowShouldClose()
		screen.Width = float32(rl.GetScreenWidth())
		screen.Height = float32(rl.GetScreenHeight())
		UpdateScreenResize(screenData, screen.Width, screen.Height)
		err := UpdatePlayer(&PLAYER, &scroll)
		if err != nil {
			LOG.Panic("%v", err.Error())
		}
		UpdateInput(input)

		rl.BeginDrawing()
		rl.ClearBackground(M_THEME.Background)
		remainder := screen

		if showFps {
			DrawDebugInfo(FONT, &remainder, M_THEME.Debug)
		}
		bounds := remainder

		titleBounds := CutTop(&bounds, line_height)
		DrawTitleShaded(titleBounds, title, M_THEME.Title)
		DrawCurrentSong(&bounds, PLAYER)

		if len(PLAYER.Playlist) > 0 {
			var currentFile = GetHighlightedScrollItem(scroll, PLAYER.Playlist)
			fpath := ""
			if currentFile != nil {
				fpath = currentFile.File.Path
			}
			width := bounds.Width - (FONT.LineSpacing*4)*4
			filePath, lines := WrapMonospaceSlow([]rune(fpath),
				FONT.CharacterWidth,
				FONT.Spacing,
				width)

			fileInfoBounds := CutBottom(&bounds, line_height*6)
			DrawFileInfo(currentFile, string(filePath), lines, fileInfoBounds)
			headerBounds := CutTop(&bounds, line_height)
			DrawScrollHeader(headerBounds, M_THEME.ScrollHeader)
			DrawScrollEx(FONT, bounds, &scroll, PLAYER.Playlist, PLAYER.SelectedSongs,
				M_THEME.Scroll, dt,
				func(itemBounds rl.Rectangle, index int, mFile *MusicFile, selected, highlighted bool) {
					DrawItem(itemBounds, index, mFile, selected, highlighted)
				},
			)
		} else {
			DrawBoundedText(FONT, "No files", bounds, TEXT_CENTER, M_THEME.Default.Text, M_THEME.Default.Bg)
		}

		rl.EndDrawing()

		ScrollInput(input, bounds, &scroll, PLAYER.Playlist, &PLAYER.SelectedSongs, MusicFile_filter_func)

		action := input.CheckAction(MUSICLIST_KEYBINDINGS, true)
		if action == NONE && globalHotkeyAction != NONE {
			action = globalHotkeyAction
		}
		err = MusicPlayerInput(action, &scroll)
		if err != nil {
			LOG.Warn("%v", err.Error())
		}
		if firstFrame { //bigass hack
			firstFrame = false
			CenterCursor(&scroll)
		}
	}
	if PLAYER.RaylibMusic != nil {
		rl.UnloadMusicStream(*PLAYER.RaylibMusic)
	}
	rl.CloseAudioDevice()
	err = saveState(PLAYER)
	if err != nil {
		panic(err)
	}
}

var MusicFile_filter_func = func(query string, items []*MusicFile) ([]*MusicFile, []int) {
	filtered := make([]*MusicFile, 0)
	indexes := make([]int, 0)
	for i, it := range items {
		if strings.Contains(strings.ToLower(it.RelativePath), strings.ToLower(query)) {
			filtered = append(filtered, it)
			indexes = append(indexes, i)
		}
	}
	return filtered, indexes
}

func UpdateScreenResize(s *ScreenData, width, height float32) {
	s.HasResized = s.PrevFrameScreen.X != width || s.PrevFrameScreen.Y != height
	if s.HasResized {
		s.PrevFrameScreen.X = width
		s.PrevFrameScreen.Y = height
	}
}

var SHADER rl.Shader
var RESOLUTIONLOC int32
var TIMELOC int32
var COLORLOC int32

var DIM_SHADER rl.Shader
var DIM_FACTORLOC int32

func DrawTitleShaded(bounds rl.Rectangle, text string, t TitleTheme) {
	rl.DrawRectangleLinesEx(bounds, 1, t.Border)
	rl.BeginShaderMode(SHADER)
	rl.DrawRectangleRec(Pad(bounds, 1), t.Bg2)
	rl.SetShaderValue(SHADER, RESOLUTIONLOC, []float32{bounds.Width * 50, bounds.Height * 50}, rl.ShaderUniformVec2)
	rl.SetShaderValue(SHADER, TIMELOC, []float32{float32(rl.GetTime())}, rl.ShaderUniformFloat)
	rl.SetShaderValue(SHADER, COLORLOC, []float32{float32(t.Bg2.R) / 255, float32(t.Bg2.G) / 255, float32(t.Bg2.B) / 255}, rl.ShaderUniformVec3)
	rl.EndShaderMode()
	DrawBoundedText(FONT, text, bounds, TEXT_CENTER, t.Text, t.Bg1)
}

type HeaderCol struct {
	Name        string
	CharCount   int
	SizePercent float32
}

func DrawScrollHeader(bounds rl.Rectangle, t BoxTextTheme) {
	width := bounds.Width - 8 //fixme: this is the scroll width
	for i, col := range PLAYER.HeaderCols {
		colWidth := width * col.SizePercent
		if i == len(PLAYER.HeaderCols)-1 {
			colWidth = bounds.Width
		}
		colBounds := CutLeft(&bounds, colWidth)
		DrawBoxEx2(colBounds, t)
		BeginScissorMode(colBounds)
		DrawBoundedText(FONT, col.Name, colBounds, TEXT_LEFT, t.Text, t.Bg)
		rl.EndScissorMode()
	}
}

func DrawItem(bounds rl.Rectangle, index int, mFile *MusicFile, selected, highlighted bool) {
	isPlaying := PLAYER.CurrentMusic != nil && index == PLAYER.CurrentMusic.Index
	t := DecideFileColor(selected, highlighted, mFile)
	cols := []string{
		fmt.Sprintf("%d", mFile.Index),
		mFile.TrackNumber,
		mFile.Title,
		mFile.Album,
		mFile.Artist,
		mFile.DurationStr,
		fmt.Sprintf("%d", mFile.PlayCount),
		mFile.RelativePath,
	}
	width := bounds.Width
	BeginScissorMode(bounds)
	for i, col := range PLAYER.HeaderCols {
		colWidth := width * col.SizePercent
		if i == len(PLAYER.HeaderCols)-1 {
			colWidth = bounds.Width
		}
		colBounds := CutLeft(&bounds, colWidth)
		bg := t.Bg
		text := t.Text
		if i == 0 {
			if isPlaying {
				bg = M_THEME.PlayingBg
				text = M_THEME.PlayingText
			}
		}
		DrawBox(colBounds, t.Thickness, bg, t.Border)
		colBounds.Width -= FONT.Spacing * 4
		BeginScissorMode(colBounds)
		DrawBoundedText(FONT, cols[i], colBounds, TEXT_LEFT, text, bg)
		rl.EndScissorMode()
	}
	rl.EndScissorMode()
}

func DrawCurrentSong(bounds *rl.Rectangle, player MusicPlayer) {
	if player.CurrentMusic == nil {
		return
	}
	volumeAndTimeBounds := CutTop(bounds, FONT.Size+4)
	width := volumeAndTimeBounds.Width / 3
	timeBounds := CutLeft(&volumeAndTimeBounds, width)
	volumeBounds := CutLeft(&volumeAndTimeBounds, width)
	modeBounds := volumeAndTimeBounds

	{ //volume
		volume := volumeBounds
		volume.Width *= player.Volume
		DrawBoxEx(volumeBounds, M_THEME.Volume.Base)
		DrawBoxEx(volume, M_THEME.Volume.Bar)
		DrawBoundedText(FONT, "VOLUME", volumeBounds, TEXT_CENTER, M_THEME.Volume.Text, M_THEME.Volume.TextBg)
	}

	{ //time
		percent := player.TimePlayed / player.TimeLength
		elapsed := timeBounds
		elapsed.Width *= percent
		DrawBoxEx(timeBounds, M_THEME.Time.Base)
		DrawBoxEx(elapsed, M_THEME.Time.Bar)
		elapsedText := "no song"
		if player.State == PLAYING {
			timePlayed := FormatDuration(player.TimePlayed)
			elapsedText = fmt.Sprintf("%s/%s", timePlayed, player.CurrentMusic.DurationStr)
		}
		DrawBoundedText(FONT, elapsedText, timeBounds, TEXT_CENTER, M_THEME.Time.Text, M_THEME.Time.TextBg)
	}

	{ //mode
		DrawBoxEx(modeBounds, M_THEME.PlayMode.Base)
		text := "NORMAL"
		switch player.PlayOrder {
		case PLAYORDER_NORMAL:
			text = "NORMAL"
		case PLAYORDER_RANDOM:
			text = "RANDOM"
		case PLAYORDER_RANDOM_NO_REPEAT:
			text = "RANDOM_NO_REPEAT"
		}
		DrawBoundedText(FONT, text, modeBounds, TEXT_CENTER, M_THEME.PlayMode.Text, M_THEME.PlayMode.TextBg)
	}

	if player.RaylibMusic != nil {
		songBounds := CutTop(bounds, FONT.Size+FONT.LineSpacing)
		DrawBoxEx(songBounds, M_THEME.CurrentSong.Base)
		text2 := fmt.Sprintf("%s - %s", player.CurrentMusic.Artist, player.CurrentMusic.Title)
		DrawBoundedText(FONT, text2, songBounds, TEXT_LEFT, M_THEME.CurrentSong.Text, M_THEME.CurrentSong.TextBg)
	}

}

func DrawFileInfo(file *MusicFile, wrappedFilepath string, wrappedLines int, bounds rl.Rectangle) {
	if file == nil {
		return
	}
	artist := fmt.Sprintf("Artist: %s", file.Artist)
	album := fmt.Sprintf("Album: %s", file.Album)
	title := fmt.Sprintf("Title: %s", file.Title)

	BeginScissorMode(bounds)
	fileInfoBoundsPadded := PadEx(bounds, FONT.Em(1), FONT.Em(1))
	pos := rl.Vector2{fileInfoBoundsPadded.X, fileInfoBoundsPadded.Y}
	DrawBoxEx2(bounds, M_THEME.CurrentSongInfo)
	DrawText(FONT, pos, wrappedFilepath, M_THEME.CurrentSongInfo.Text)
	pos.Y += line_height * float32(wrappedLines)
	DrawText(FONT, pos, artist, M_THEME.CurrentSongInfo.Text)
	pos.Y += line_height
	DrawText(FONT, pos, album, M_THEME.CurrentSongInfo.Text)
	pos.Y += line_height
	DrawText(FONT, pos, title, M_THEME.CurrentSongInfo.Text)
	rl.EndScissorMode()
}

type MusicState int

const (
	STOPPED MusicState = iota
	PLAYING
	PAUSED
)

const (
	STOP = iota + LAST_ACTION
	PLAY_PAUSE
	NEXT
	PREVIOUS
	FORWARD
	REWIND
	NEXT_RANDOM
	NEXT_RANDOM_NO_REPEAT
	VOLUME_UP
	VOLUME_DOWN
	CHANGE_PLAYORDER_NORMAL
	CHANGE_PLAYORDER_RANDOM
	CHANGE_PLAYORDER_RANDOM_NO_REPEAT
	DELETE
	COPY_FULL_FILE_PATH
	REFRESH
)

var MUSICLIST_KEYBINDINGS = []KeyBind{
	{CLICK, rl.KeyEnter},
	{CANCEL, rl.KeyEscape},
	{PLAY_PAUSE, MOD_CTRL | rl.KeyEnter},
	{STOP, MOD_CTRL | rl.KeySpace},
	{NEXT, MOD_CTRL | rl.KeyRight},
	{PREVIOUS, MOD_CTRL | rl.KeyLeft},
	{FORWARD, rl.KeyRight},
	{REWIND, rl.KeyLeft},
	{VOLUME_DOWN, MOD_CTRL | rl.KeyDown},
	{VOLUME_UP, MOD_CTRL | rl.KeyUp},
	{NEXT_RANDOM, MOD_CTRL | rl.KeyR},
	{NEXT_RANDOM_NO_REPEAT, MOD_CTRL | MOD_SHIFT | rl.KeyR},
	{CHANGE_PLAYORDER_NORMAL, rl.KeyOne},
	{CHANGE_PLAYORDER_RANDOM, rl.KeyTwo},
	{CHANGE_PLAYORDER_RANDOM_NO_REPEAT, rl.KeyThree},
	{DELETE, MOD_CTRL | rl.KeyDelete},
	{COPY_FULL_FILE_PATH, MOD_CTRL | rl.KeyC},
	{REFRESH, rl.KeyF5},
}

func MusicPlayerInput(action Action, scroll *Scroll[*MusicFile]) error {
	if action != NONE {
		scroll.Dirty = true
	}
	switch action {
	case CLICK:
		err := PlaySong(&PLAYER, scroll)
		if err != nil {
			return err
		}
	case CANCEL:
		//nothing
	case PLAY_PAUSE:
		err := PlayOrPauseSong(&PLAYER, scroll)
		if err != nil {
			return err
		}
	case STOP:
		StopSong(&PLAYER)
	case NEXT:
		index, err := NextSong(&PLAYER)
		if err != nil {
			return err
		}
		scroll.Cursor = index
		CenterCursor(scroll)
	case PREVIOUS:
		index := PrevSong(&PLAYER, scroll)
		scroll.Cursor = index
		CenterCursor(scroll)
	case FORWARD:
		ForwardSong(&PLAYER)
	case REWIND:
		RewindSong(&PLAYER)
	case NEXT_RANDOM_NO_REPEAT:
		index := NextRandomNoRepeatSong(&PLAYER)
		scroll.Cursor = index
		CenterCursor(scroll)
	case NEXT_RANDOM:
		index := NextRandomSong(&PLAYER)
		scroll.Cursor = index
		CenterCursor(scroll)
	case VOLUME_UP:
		VolumeUp(&PLAYER)
	case VOLUME_DOWN:
		VolumeDown(&PLAYER)
	case CHANGE_PLAYORDER_NORMAL:
		PLAYER.PlayOrder = PLAYORDER_NORMAL
	case CHANGE_PLAYORDER_RANDOM:
		PLAYER.PlayOrder = PLAYORDER_RANDOM
	case CHANGE_PLAYORDER_RANDOM_NO_REPEAT:
		PLAYER.PlayOrder = PLAYORDER_RANDOM_NO_REPEAT
	case DELETE:
		err := DeleteSongs(&PLAYER, scroll)
		if err != nil {
			return err
		}
	case COPY_FULL_FILE_PATH:
		SongFilepathToClipboard(&PLAYER, scroll)
	case REFRESH:
		err := RefreshPlaylist(&PLAYER, scroll)
		if err != nil {
			return err
		}
	}
	return nil
}

type PlayOrder int

const (
	PLAYORDER_NORMAL PlayOrder = iota
	PLAYORDER_RANDOM
	PLAYORDER_RANDOM_NO_REPEAT
)

func Push[T any](x T, arr *[]T) {
	if arr == nil {
		a := make([]T, 0)
		arr = &a
	}
	*arr = append(*arr, x)
}
func Pop[T any](arr *[]T) T {
	x := (*arr)[len(*arr)-1]
	*arr = (*arr)[:len(*arr)-1]
	return x
}

func PlaySong(player *MusicPlayer, scroll *Scroll[*MusicFile]) error {
	song := GetHighlightedScrollItem(*scroll, player.Playlist)
	if song == nil {
		return fmt.Errorf("invalid song")
	}
	err := playSongAt(player, song.Index)
	if err != nil {
		return err
	}
	if IsFilterActive(*scroll) {
		foundIdx := -1
		for i, idx := range scroll.FilterIndexes {
			if idx == song.Index {
				foundIdx = i
				break
			}
		}
		if foundIdx > 0 {
			scroll.FilterCursor = foundIdx
			CenterCursor(scroll)
		}
	} else {
		scroll.Cursor = song.Index
		CenterCursor(scroll)
	}
	return nil
}
func PlayOrPauseSong(player *MusicPlayer, scroll *Scroll[*MusicFile]) error {
	if len(player.Playlist) == 0 {
		return nil
	}
	if player.CurrentMusic == nil {
		PlaySong(player, scroll)
	} else {
		switch player.State {
		case PLAYING:
			if player.RaylibMusic != nil {
				rl.PauseMusicStream(*player.RaylibMusic)
			}
			player.State = PAUSED
			return nil
		case PAUSED:
			if player.RaylibMusic != nil {
				rl.ResumeMusicStream(*player.RaylibMusic)
			}
			player.State = PLAYING
			return nil
		}
	}
	return nil
}
func UpdatePlayer(player *MusicPlayer, scroll *Scroll[*MusicFile]) error {
	if player.RaylibMusic != nil {
		rl.UpdateMusicStream(*player.RaylibMusic)
		player.TimePlayed = rl.GetMusicTimePlayed(*player.RaylibMusic)
		player.TimeLength = rl.GetMusicTimeLength(*player.RaylibMusic)
		if player.TimePlayed >= player.TimeLength-0.05 {
			player.Playcount[player.CurrentMusic.RelativePath] += 1
			player.CurrentMusic.PlayCount += 1
			nextIndex, err := NextSong(player)
			if err != nil {
				return err
			}
			scroll.Cursor = nextIndex
			CenterCursor(scroll)
			scroll.Dirty = true
			player.TimePlayed = 0
			saveState(*player)
		}
	}
	return nil
}
func StopSong(player *MusicPlayer) {
	if player.State != STOPPED {
		if player.RaylibMusic != nil {
			rl.StopMusicStream(*player.RaylibMusic)
		}
		player.State = STOPPED
	}
}
func NextSong(player *MusicPlayer) (_index int, _err error) {
	if player.RaylibMusic != nil {
		rl.StopMusicStream(*player.RaylibMusic)
	}
	if player.CurrentMusic != nil {
		Push(player.CurrentMusic.Index, &player.PlayedSongs)
	}
	nextIndex := 0
	switch player.PlayOrder {
	case PLAYORDER_NORMAL:
		if player.CurrentMusic == nil {
			if len(player.Playlist) > 0 {
				nextIndex = 0
			}
		} else {
			if player.CurrentMusic.Index < len(player.Playlist)-1 {
				nextIndex = player.CurrentMusic.Index + 1
			}
		}
	case PLAYORDER_RANDOM:
		nextIndex = int(rl.GetRandomValue(0, int32(len(player.Playlist)-1)))
	case PLAYORDER_RANDOM_NO_REPEAT:
		if len(player.RemainingUnique) == 0 {
			player.RemainingUnique = make([]int, len(player.Playlist))
			for i := range player.RemainingUnique {
				player.RemainingUnique[i] = i
			}
			rand.Shuffle(len(player.Playlist), func(i, j int) {
				player.RemainingUnique[i], player.RemainingUnique[j] = player.RemainingUnique[j], player.RemainingUnique[i]
			})
		}
		nextIndex = Pop(&player.RemainingUnique)
	}
	err := playSongAt(player, nextIndex)
	if err != nil {
		return nextIndex, err
	}
	return nextIndex, nil
}
func PrevSong(player *MusicPlayer, scroll *Scroll[*MusicFile]) (_index int) {
	if len(player.PlayedSongs) == 0 {
		return
	}
	if player.RaylibMusic != nil {
		rl.StopMusicStream(*player.RaylibMusic)
	}
	nextIndex := 0
	if player.PlayOrder == PLAYORDER_NORMAL {
		nextIndex = Clamp(scroll.Cursor-1, 0, len(player.Playlist))
	} else {
		nextIndex = Pop(&player.PlayedSongs)
	}
	playSongAt(player, nextIndex)
	return nextIndex
}
func NextRandomSong(player *MusicPlayer) (_index int) {
	if player.RaylibMusic != nil {
		rl.StopMusicStream(*player.RaylibMusic)
	}
	if player.CurrentMusic != nil {
		Push(player.CurrentMusic.Index, &player.PlayedSongs)
	}
	nextIndex := int(rl.GetRandomValue(0, int32(len(player.Playlist)-1)))
	playSongAt(player, nextIndex)
	return nextIndex
}
func NextRandomNoRepeatSong(player *MusicPlayer) (_index int) {
	if player.RaylibMusic != nil {
		rl.StopMusicStream(*player.RaylibMusic)
	}
	if player.CurrentMusic != nil {
		Push(player.CurrentMusic.Index, &player.PlayedSongs)
	}
	if len(player.RemainingUnique) == 0 {
		player.RemainingUnique = make([]int, len(player.Playlist))
		for i := range player.RemainingUnique {
			player.RemainingUnique[i] = i
		}
		rand.Shuffle(len(player.Playlist), func(i, j int) { player.Playlist[i], player.Playlist[j] = player.Playlist[j], player.Playlist[i] })
	}
	nextIndex := Pop(&player.RemainingUnique)
	playSongAt(player, nextIndex)
	return nextIndex
}
func VolumeUp(player *MusicPlayer) {
	player.Volume = Min(player.Volume+0.05, 1)
	if player.RaylibMusic != nil {
		rl.SetMusicVolume(*player.RaylibMusic, player.Volume)
	}
}
func VolumeDown(player *MusicPlayer) {
	player.Volume = Max(player.Volume-0.05, 0)
	if player.RaylibMusic != nil {
		rl.SetMusicVolume(*player.RaylibMusic, player.Volume)
	}
}
func ForwardSong(player *MusicPlayer) {
	if player.RaylibMusic == nil {
		return
	}
	currentTime := rl.GetMusicTimePlayed(*player.RaylibMusic)
	maxTime := rl.GetMusicTimeLength(*player.RaylibMusic)
	newTime := Min(currentTime+4, maxTime)
	rl.SeekMusicStream(*player.RaylibMusic, newTime)
}
func RewindSong(player *MusicPlayer) {
	if player.RaylibMusic == nil {
		return
	}
	currentTime := rl.GetMusicTimePlayed(*player.RaylibMusic)
	newTime := Max(currentTime-4, 0.01)
	rl.SeekMusicStream(*player.RaylibMusic, newTime)
}
func DeleteSongs(player *MusicPlayer, scroll *Scroll[*MusicFile]) error {
	//todo: delete multiple songs, depending on what's selected
	//needs a confirmation modal...maybe
	song := GetHighlightedScrollItem(*scroll, player.Playlist)
	if player.CurrentMusic != nil && player.CurrentMusic.File.Path == song.File.Path {
		StopSong(player)
	}
	err := DeleteFileTrashbin(song.File.Path)
	if err != nil {
		return err
	}
	player.Playlist = slices.Delete(player.Playlist, scroll.Cursor, scroll.Cursor+1)
	if len(scroll.FilterItems) != 0 {
		scroll.FilterItems = slices.Delete(scroll.FilterItems, scroll.FilterCursor, scroll.FilterCursor+1)
	}
	musicfiles, headerCols, err := LoadMusicFiles(player.PlaylistPath)
	if err != nil {
		return err
	}
	player.HeaderCols = headerCols
	player.Playlist = musicfiles
	scroll.Cursor = Clamp(scroll.Cursor, 0, len(player.Playlist)-1)
	if len(scroll.FilterItems) != 0 {
		scroll.FilterCursor = Clamp(scroll.FilterCursor, 0, len(scroll.FilterItems)-1)
	}
	return nil
}
func RefreshPlaylist(player *MusicPlayer, scroll *Scroll[*MusicFile]) error {
	musicfiles, headerCols, err := LoadMusicFiles(player.PlaylistPath)
	if err != nil {
		return err
	}
	player.HeaderCols = headerCols
	player.Playlist = musicfiles
	//scroll.Cursor = 0 //TODO: cursor should behave better
	return nil
}

func SongFilepathToClipboard(player *MusicPlayer, scroll *Scroll[*MusicFile]) {
	song := GetHighlightedScrollItem(*scroll, player.Playlist)
	rl.SetClipboardText(song.File.Path)
}

// type CustomPCMSource struct {
// 	Decoder    *FLACDecoder
// 	SampleRate uint32
// 	Channels   uint32
// 	Stream     rl.AudioStream
// 	PCM        []float32
// }

func playSongAt(player *MusicPlayer, index int) error {
	if len(player.Playlist) == 0 {
		return nil
	}
	nextSong := player.Playlist[index]
	player.CurrentMusic = nextSong

	var music rl.Music
	LOG.Info("playing: %s", player.CurrentMusic.RelativePath)
	music = rl.LoadMusicStream(nextSong.File.Path)
	player.RaylibMusic = &music
	rl.PlayMusicStream(*player.RaylibMusic)
	rl.SetMusicVolume(*player.RaylibMusic, player.Volume)
	player.State = PLAYING
	newTitle := fmt.Sprintf("%s - %s", nextSong.Artist, nextSong.Title)
	rl.SetWindowTitle(newTitle)
	return nil
}

type MusicFile struct {
	Index        int
	File         File
	Album        string
	Artist       string
	Title        string
	TrackNumber  string
	Fullname     string
	RelativePath string
	DurationStr  string
	Duration     time.Duration
	PlayCount    int
}

type File struct {
	FileInfo    fs.FileInfo
	DisplayName string
	Path        string
	Ext         string

	SizeStr    string
	ModTimeStr string

	Size        int64
	ModTime     time.Time
	Permissions string
	Uid         string
	Gid         string

	Type FileType
}

type FileType int

const (
	DIR FileType = iota
	FILE
	FILE_SYMLINK
	FOLDER_SYMLINK
	BROKEN_SYMLINK
	HIDDEN_FILE
	HIDDEN_FOLDER
)

func DecideFileColor(selected, highlighted bool, mFile *MusicFile) BoxTextTheme {
	t := BoxTextTheme{}
	file := mFile.File
	fileColors := FileTypeColors(file.Ext)
	if highlighted {
		if selected {
			t.Text = M_THEME.HighlightedFileText
			t.Bg = M_THEME.HighlightedFileBg
		} else {
			t.Text = fileColors.HighlightedText
			t.Bg = fileColors.HighlightedBg
		}
	} else {
		if selected {
			t.Text = M_THEME.SelectedFileText
			t.Bg = M_THEME.SelectedFileBg
		} else {
			t.Text = fileColors.Text
			t.Bg = fileColors.Bg
		}
	}
	return t
}

func FileTypeColors(ext string) FileTypeColor {
	val, ok := M_THEME.FileColors[ext]
	if !ok {
		return M_THEME.FileColors[".mp3"]
	}
	return val
}

func isMusicExtension(ext string) bool {
	lower := strings.ToLower(ext)
	switch lower {
	case ".mp3", ".wav", ".ogg", ".webm", ".mp4", ".flac": //,".vgz",".spc":
		return true
	default:
		return false
	}
}
func ReadFolderRecursive(dirPath string) ([]File, error) {
	path, err := ExpandHome(dirPath)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0)
	err = filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			newFile := *NewFile(path, info)
			files = append(files, newFile)
		}
		return nil
	})
	return files, err
}

func NewFile(fullpath string, fileInfo os.FileInfo) *File {
	ftype := getFileType(fileInfo, fullpath)
	displayName := fileInfo.Name()
	ext := filepath.Ext(displayName)

	switch ftype {
	case DIR, FOLDER_SYMLINK, HIDDEN_FOLDER:
		displayName += "/"
	}

	sysInfo := fileInfo.Sys().(*syscall.Stat_t)
	f := &File{
		FileInfo:    fileInfo,
		DisplayName: displayName,
		Ext:         ext,
		Path:        fullpath,
		Type:        ftype,
		Size:        fileInfo.Size(),
		Gid:         fmt.Sprintf("%d", sysInfo.Gid),
		Uid:         fmt.Sprintf("%d", sysInfo.Uid),
		ModTime:     fileInfo.ModTime(),
		Permissions: fileInfo.Mode().String(),
	}
	return f
}

func getFileType(fileInfo os.FileInfo, filePath string) FileType {
	var _type FileType
	if fileInfo.IsDir() {
		_type = DIR
	} else if fileInfo.Mode()&os.ModeSymlink != 0 {
		targetInfo, err := os.Stat(filePath)
		if err == nil {
			if targetInfo.IsDir() {
				_type = FOLDER_SYMLINK
			} else {
				_type = FILE_SYMLINK
			}
		} else {
			_type = BROKEN_SYMLINK
		}
	} else {
		_type = FILE
	}

	if strings.HasPrefix(fileInfo.Name(), ".") {
		switch _type {
		case DIR, FOLDER_SYMLINK:
			return HIDDEN_FOLDER
		case FILE, FILE_SYMLINK, BROKEN_SYMLINK:
			return HIDDEN_FILE
		}
	}
	return _type
}

func CreateFolderIfNotExists(path string) error {
	if !FileExists(path) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

type MusicPlayerNonPointer struct {
	CurrentMusicPath string
	State            MusicState
	Volume           float32
	TimePlayed       float32
	TimeLength       float32

	PlaylistPath string
	HeaderCols   []HeaderCol
	PlayOrder    PlayOrder
	Playcount    map[string]int
}

func saveState(program MusicPlayer) error {
	p := MusicPlayerNonPointer{
		State:      program.State,
		Volume:     program.Volume,
		TimePlayed: program.TimePlayed,
		TimeLength: program.TimeLength,

		PlaylistPath: program.PlaylistPath,
		HeaderCols:   program.HeaderCols,
		PlayOrder:    program.PlayOrder,
		Playcount:    program.Playcount,
	}
	if program.CurrentMusic != nil {
		p.CurrentMusicPath = program.CurrentMusic.File.Path
	}
	path, err := ExpandHome(config_folder)
	if err != nil {
		return err
	}
	err = CreateFolderIfNotExists(path)
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(path, state_filename))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(p)
}

func LoadState() (*MusicPlayerNonPointer, error) {
	path, err := ExpandHome(config_folder)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(path, state_filename))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	state := MusicPlayerNonPointer{}
	encoder := gob.NewDecoder(file)
	err = encoder.Decode(&state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func LoadMusicFiles(path string) ([]*MusicFile, []HeaderCol, error) {
	type Data struct {
		Median     int
		MedianVals map[int]int
		Average    int
		Total      int
	}

	files, err := ReadFolderRecursive(path)
	if err != nil {
		return nil, nil, err
	}
	musicFiles := make([]*MusicFile, 0)
	var wg sync.WaitGroup
	wg.Add(len(files))
	for _, _file := range files {
		go func(file File) {
			defer wg.Done()
			if !isMusicExtension(file.Ext) {
				return
			}
			rel := strings.TrimPrefix(file.Path, path)
			mus := &MusicFile{
				File:         file,
				Fullname:     file.DisplayName,
				RelativePath: rel,
				PlayCount:    0,
				Album:        "?",
				Artist:       "?",
				Title:        file.DisplayName,
				TrackNumber:  "?",
				DurationStr:  "?",
			}

			f, err := os.Open(file.Path)
			if err != nil {
				//fmt.Printf("couldn't get tags for %s, reason: %s\n", mus.RelativePath, err.Error())
				musicFiles = append(musicFiles, mus)
				return
			}
			dur, err := ReadDurationFile(f)
			if err != nil {
				//fmt.Printf("couldn't read duration for %v: %v\n", file.Path, err.Error())
			}
			f.Seek(0, 0)
			meta, err := tag.ReadFrom(f)
			if err != nil {
				//fmt.Printf("couldn't get tags for %s, reason: %s\n", mus.RelativePath, err.Error())
				musicFiles = append(musicFiles, mus)
				return
			}
			f.Close()
			if len(meta.Album()) > 0 {
				mus.Album = meta.Album()
			}
			if len(meta.Artist()) > 0 {
				mus.Artist = meta.Artist()
			}
			if len(meta.Title()) > 0 {
				mus.Title = meta.Title()
			}
			mus.Duration = dur
			mus.DurationStr = FormatDuration(float32(dur.Seconds()))
			mus.PlayCount = PLAYER.Playcount[rel]
			track, trackTotal := meta.Track()
			trackStr := ""
			if track == 0 && trackTotal == 0 {
				trackStr = "?"
			} else if track == 0 {
				trackStr = fmt.Sprintf("?/%d", trackTotal)
			} else if trackTotal == 0 {
				trackStr = fmt.Sprintf("%d/?", track)
			} else {
				trackStr = fmt.Sprintf("%d/%d", track, trackTotal)
			}
			mus.TrackNumber = trackStr

			musicFiles = append(musicFiles, mus)
		}(_file)
	}
	wg.Wait()

	slices.SortStableFunc(musicFiles, func(a, b *MusicFile) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})

	setDataTotal := func(sets *map[string]Data, key string, value int) {
		if _, ok := (*sets)[key]; !ok {
			(*sets)[key] = Data{MedianVals: make(map[int]int, 0)}
		}
		d := (*sets)[key]
		d.MedianVals[value] += 1
		d.Total += value
		(*sets)[key] = d
	}
	dataSet := make(map[string]Data, 0)
	for i, m := range musicFiles {
		m.Index = i
		setDataTotal(&dataSet, "album", len(m.Album))
		setDataTotal(&dataSet, "artist", len(m.Artist))
		setDataTotal(&dataSet, "title", len(m.Title))
		setDataTotal(&dataSet, "fullname", len(m.Fullname))
		setDataTotal(&dataSet, "tracknumber", len(m.TrackNumber))
		setDataTotal(&dataSet, "duration", len(m.DurationStr))
		setDataTotal(&dataSet, "path", len(m.File.Path))
		setDataTotal(&dataSet, "playcount", len(fmt.Sprintf("%d", m.PlayCount)))
	}
	for key := range dataSet {
		d := dataSet[key]
		d.Average = d.Total / len(musicFiles)
		medianAmount := 1
		medianKey := 1
		for k, v := range d.MedianVals {
			if v > medianAmount {
				medianAmount = v
				medianAmount = k
			}
		}
		d.Median = medianKey
		dataSet[key] = d
	}
	var headerCols = make([]HeaderCol, 0)
	indexCharCount := len(fmt.Sprintf("%d", len(musicFiles)))
	tracknumberLen := dataSet["tracknumber"].Average + 2
	titleLen := dataSet["title"].Average
	albumLen := dataSet["album"].Average
	artistLen := dataSet["artist"].Average
	durationLen := dataSet["duration"].Average
	playcountLen := dataSet["playcount"].Average + 2
	pathLen := 2 //dataSet["path"].Average
	totalChars := float32(indexCharCount)
	totalChars += float32(tracknumberLen)
	totalChars += float32(titleLen)
	totalChars += float32(albumLen)
	totalChars += float32(artistLen)
	totalChars += float32(durationLen)
	totalChars += float32(playcountLen)
	totalChars += float32(pathLen)

	indexPercent := float32(indexCharCount) / totalChars
	tracknumberPercent := float32(tracknumberLen) / totalChars
	titlePercent := float32(titleLen) / totalChars
	albumPercent := float32(albumLen) / totalChars
	artistPercent := float32(artistLen) / totalChars
	durationPercent := float32(durationLen) / totalChars
	playcountPercent := float32(playcountLen) / totalChars
	pathPercent := float32(pathLen) / totalChars

	headerCols = append(headerCols, HeaderCol{"index", 4, indexPercent})
	headerCols = append(headerCols, HeaderCol{"tracknumber", tracknumberLen, tracknumberPercent})
	headerCols = append(headerCols, HeaderCol{"title", titleLen, titlePercent})
	headerCols = append(headerCols, HeaderCol{"album", albumLen, albumPercent})
	headerCols = append(headerCols, HeaderCol{"artist", artistLen, artistPercent})
	headerCols = append(headerCols, HeaderCol{"duration", durationLen, durationPercent})
	headerCols = append(headerCols, HeaderCol{"playcount", playcountLen, playcountPercent})
	headerCols = append(headerCols, HeaderCol{"path", pathLen, pathPercent})
	return musicFiles, headerCols, nil
}

// todo: this sucks man
func DeleteFileTrashbin(fullpath string) error {
	trash := os.Getenv("XDG_DATA_HOME")
	if trash == "" {
		trash = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	trashDir := filepath.Join(trash, "Trash")
	filesDir := filepath.Join(trashDir, "files")
	infoDir := filepath.Join(trashDir, "info")
	err := CreateFolderIfNotExists(filesDir)
	if err != nil {
		return fmt.Errorf(
			"couldn't find default trash folder path, tried creating default trash folder on %s\n: %w", filesDir, err)
	}
	err = CreateFolderIfNotExists(infoDir)
	if err != nil {
		return fmt.Errorf(
			"couldn't find default metadata trash folder path, tried creating default metadata trash folder on %s\n: %w", infoDir, err)
	}

	absPath, err := filepath.Abs(fullpath)
	if err != nil {
		return err
	}
	pathForInfo := absPath
	rel, err := filepath.Rel(filepath.Dir(trashDir), absPath)
	if err == nil && !strings.HasPrefix(rel, "..") {
		pathForInfo = rel
	}
	base := filepath.Base(absPath)
	newPath := filepath.Join(filesDir, base)

	i := 1
	for FileExists(newPath) {
		newName := fmt.Sprintf("%s (%d)", base, i)
		newPath = filepath.Join(filesDir, newName)
		i++
	}
	infoName := filepath.Base(newPath) + ".trashinfo"
	infoPath := filepath.Join(infoDir, infoName)

	var b strings.Builder
	b.Grow(len(pathForInfo))

	for _, c := range pathForInfo {
		switch {
		case c == '%':
			b.WriteString("%%")
		case c <= ' ':
			fmt.Fprintf(&b, "%%%02X", c)
		default:
			b.WriteRune(c)
		}
	}
	pathForInfo = b.String()

	data := fmt.Sprintf(
		"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		pathForInfo,
		time.Now().Local().Format("2006-01-02T15:04:05"),
	)
	if err := os.WriteFile(infoPath, []byte(data), 0644); err != nil {
		return fmt.Errorf("tried to write trash metadata on %s : \n %w", infoPath, err)
	}

	if err := os.Rename(absPath, newPath); err != nil {
		os.Remove(infoPath)
		return err
	}
	return nil
}

func FormatDuration(seconds float32) string {
	total := int(seconds + 0.5)

	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}

	return fmt.Sprintf("%02d:%02d", m, s)
}

type CyclicalSoundState int

const (
	CYC_STOP CyclicalSoundState = iota
	CYC_START
	CYC_CHECK
	CYC_PLAY
	CYC_PLAYING
)

type CyclicalSound struct {
	Sound      rl.Music
	State      CyclicalSoundState
	CycleTime  time.Duration
	Start      time.Time
	TimePlayed float32
	TimeLength float32
}

func NewCyclicalSound(data []byte, cycleTime time.Duration) *CyclicalSound {
	mus := rl.LoadMusicStreamFromMemory(".mp3", data, int32(len(data)))
	cyc := &CyclicalSound{
		Sound:     mus,
		State:     CYC_STOP,
		CycleTime: cycleTime,
		Start:     time.Now(),
	}
	cyc.TimeLength = rl.GetMusicTimeLength(cyc.Sound)
	return cyc
}

func UpdateCycle(c *CyclicalSound) {
	switch c.State {
	case CYC_STOP:
		if rl.IsMusicStreamPlaying(c.Sound) {
			rl.StopMusicStream(c.Sound)
		}
	case CYC_START:
		c.Start = time.Now()
		c.State = CYC_CHECK
	case CYC_CHECK:
		elapsed := time.Since(c.Start)
		if elapsed >= c.CycleTime {
			c.State = CYC_PLAY
		}
	case CYC_PLAY:
		rl.PlayMusicStream(c.Sound)
		c.State = CYC_PLAYING
	case CYC_PLAYING:
		rl.UpdateMusicStream(c.Sound)
		c.TimePlayed = rl.GetMusicTimePlayed(c.Sound)
		if c.TimePlayed >= c.TimeLength-0.05 {
			rl.StopMusicStream(c.Sound)
			c.State = CYC_START
			c.TimePlayed = 0
		}
	}
}
