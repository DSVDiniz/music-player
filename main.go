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

	cui "github.com/dsvdiniz/cutterui"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	tag "github.com/dhowden/tag"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func ListenHotkeys(c chan cui.Action) {
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

var FONT cui.Font

var THEME = cui.BoxTheme{Thickness: 1, Text: rl.White, Border: rl.White, Bg: rl.Black}

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

	hotkeyChan := make(chan cui.Action)

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

	go cui.MonitorMemory()
	rl.SetTraceLogLevel(rl.LogNone)
	rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(800, 600, "music-player")
	defer rl.CloseWindow()
	rl.SetExitKey(rl.KeyNull)
	rl.InitAudioDevice()

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
		//fixme: resuming songs after shutdown is not working
		PLAYER.CurrentMusic = currentMusic
		if PLAYER.CurrentMusic != nil {
			mus := rl.LoadMusicStream(PLAYER.CurrentMusic.File.Path)
			PLAYER.RaylibMusic = &mus
			PLAYER.State = PAUSED
		}
		//if len(PLAYER.HeaderCols) == 0 {
		PLAYER.HeaderCols = headerCols
		//}
		PLAYER.Playlist = musicfiles
	}

	SHADER = rl.LoadShader("shaders/shader.vs", "shaders/shader.fs")
	RESOLUTIONLOC = rl.GetShaderLocation(SHADER, "resolution")
	TIMELOC = rl.GetShaderLocation(SHADER, "time")
	COLORLOC = rl.GetShaderLocation(SHADER, "uColor")

	DIM_SHADER = rl.LoadShader("shaders/dim_shader.vs", "shaders/dim_shader.fs")
	DIM_FACTORLOC = rl.GetShaderLocation(DIM_SHADER, "dimFactor")
	//fontPath := "roboto.ttf"
	fontPath, err := cui.GetSystemFont()
	if err != nil {
		LOG.Warn("couldn't load system font: %s", err)
	}
	const font_size = 20
	const codepointSize = 20000
	FONT = cui.NewFont(fontPath, font_size, codepointSize)
	FONT.IsMonospace = true
	defer rl.UnloadFont(FONT.Font)

	scroll := cui.Scroll[*MusicFile]{
		ItemHeight: FONT.Size,
		FilterSLE:  cui.NewSingleLineEditor(),
	}

	playlistTitle := fmt.Sprintf("%s", PLAYER.PlaylistPath)
	title := fmt.Sprintf("music-player - %s", playlistTitle)

	input := cui.NewInput()
	screenData := &ScreenData{}
	exit := rl.WindowShouldClose()
	showFps := false
	screen := rl.Rectangle{}

	cycle := toDuration(170)
	rumbleSound := NewCyclicalSound(soundData, cycle)
	for !exit {
		if noise {
			UpdateCycle(rumbleSound)
			if PLAYER.State == PLAYING && rumbleSound.State != CYC_STOP {
				rumbleSound.State = CYC_STOP
			} else if (PLAYER.State == STOPPED || PLAYER.State == PAUSED) && (rumbleSound.State == CYC_STOP) {
				rumbleSound.State = CYC_START
			}
		}

		globalHotkeyAction := cui.NONE
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
		cui.UpdateInput(input)

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		remainder := screen

		if showFps {
			DrawDebugInfo(&remainder)
		}
		bounds := remainder

		titleBounds := cui.CutTop(&bounds, FONT.Size+FONT.LineSpacing)
		DrawTitleShaded(titleBounds, title, rl.Blue, THEME.Text, THEME.Bg)
		DrawCurrentSong(&bounds, PLAYER)

		if len(PLAYER.Playlist) > 0 {
			var currentFile = cui.GetHighlightedScrollItem(scroll, PLAYER.Playlist)
			fpath := ""
			if currentFile != nil {
				fpath = currentFile.File.Path
			}
			width := bounds.Width - (FONT.LineSpacing*4)*4
			filePath, lines := cui.WrapMonospaceSlow(
				[]rune(fpath),
				FONT.CharacterWidth,
				FONT.Spacing,
				width)
			lineHeight := FONT.LineSpacing + FONT.Size
			fileInfoBounds := cui.CutBottom(&bounds, lineHeight*6)
			DrawFileInfo(currentFile, string(filePath), lines, fileInfoBounds)
			headerBounds := cui.CutTop(&bounds, FONT.Size+FONT.LineSpacing)
			DrawScrollHeader(headerBounds)
			cui.DrawScroll(FONT, bounds, &scroll, PLAYER.Playlist, PLAYER.SelectedSongs, rl.Red, THEME, dt,
				func(itemBounds rl.Rectangle, index int, mFile *MusicFile, selected, highlighted bool) {
					DrawItem(itemBounds, index, mFile, selected, highlighted)
				},
			)
		} else {
			cui.DrawBoundedText(FONT, "No files", bounds, cui.TEXT_CENTER, rl.White, rl.Black)
		}

		rl.EndDrawing()

		cui.ScrollInput(input, bounds, &scroll, PLAYER.Playlist, &PLAYER.SelectedSongs, MusicFile_filter_func)

		action := input.CheckAction(MUSICLIST_KEYBINDINGS, true)
		if action == cui.NONE && globalHotkeyAction != cui.NONE {
			action = globalHotkeyAction
		}
		err = Input(action, &scroll)
		if err != nil {
			LOG.Warn("%v", err.Error())
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
func DrawDebugInfo(bounds *rl.Rectangle) {
	const padding = 10
	statusBarText := fmt.Sprintf("FPS: %d FrameTime:%.2fms Alloc: %vMiB TotalAlloc: %vMiB Sys: %vMiB NumGC: %v",
		rl.GetFPS(), rl.GetFrameTime()*1000, cui.Alloc, cui.TotalAlloc, cui.Sys, cui.NumGC)
	text, lines := cui.WrapMonospaceSlow(
		[]rune(statusBarText),
		FONT.CharacterWidth, FONT.Spacing, bounds.Width-padding*2,
	)
	fpsInfoArea := cui.CutTop(bounds, FONT.Padding*float32(lines))
	rl.DrawRectangleRec(fpsInfoArea, THEME.Bg)
	rl.DrawRectangleLinesEx(fpsInfoArea, 1, THEME.Border)
	textPos := rl.Vector2{
		fpsInfoArea.X + padding,
		fpsInfoArea.Y + (fpsInfoArea.Height-(FONT.Size*float32(lines)))/2,
	}
	cui.DrawText(FONT, textPos, string(text), THEME.Text)
}

var SHADER rl.Shader
var RESOLUTIONLOC int32
var TIMELOC int32
var COLORLOC int32

var DIM_SHADER rl.Shader
var DIM_FACTORLOC int32

func DrawTitleShaded(bounds rl.Rectangle, text string, shader, txtColor, txtBg rl.Color) {
	rl.DrawRectangleLinesEx(bounds, 1, rl.White)
	rl.BeginShaderMode(SHADER)
	rl.DrawRectangleRec(cui.Pad(bounds, 1), shader)
	rl.SetShaderValue(SHADER, RESOLUTIONLOC, []float32{bounds.Width * 50, bounds.Height * 50}, rl.ShaderUniformVec2)
	rl.SetShaderValue(SHADER, TIMELOC, []float32{float32(rl.GetTime())}, rl.ShaderUniformFloat)
	rl.SetShaderValue(SHADER, COLORLOC, []float32{float32(shader.R) / 255, float32(shader.G) / 255, float32(shader.B) / 255}, rl.ShaderUniformVec3)
	rl.EndShaderMode()
	cui.DrawBoundedText(FONT, text, bounds, cui.TEXT_CENTER, txtColor, txtBg)
}

type HeaderCol struct {
	Name        string
	CharCount   int
	SizePercent float32
}

func DrawScrollHeader(bounds rl.Rectangle) {
	t := cui.BoxTheme{1, rl.White, rl.White, rl.Gray}
	width := bounds.Width - 8 //fixme: this is the scroll width
	for i, col := range PLAYER.HeaderCols {
		colWidth := width * col.SizePercent
		if i == len(PLAYER.HeaderCols)-1 {
			colWidth = bounds.Width
		}
		colBounds := cui.CutLeft(&bounds, colWidth)
		cui.DrawBoxEx(colBounds, t)
		cui.BeginScissorMode(colBounds)
		cui.DrawBoundedText(FONT, col.Name, colBounds, cui.TEXT_LEFT, t.Text, t.Bg)
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
	cui.BeginScissorMode(bounds)
	rl.DrawRectangleRec(bounds, rl.Red)
	for i, col := range PLAYER.HeaderCols {
		colWidth := width * col.SizePercent
		if i == len(PLAYER.HeaderCols)-1 {
			colWidth = bounds.Width
		}
		colBounds := cui.CutLeft(&bounds, colWidth)
		color := t.Bg
		tColor := t.Text
		if i == 0 {
			if isPlaying {
				color = rl.Red
				tColor = rl.White
			}
		}
		cui.DrawBox(colBounds, t.Thickness, color, t.Border)
		colBounds.Width -= FONT.Spacing * 4
		cui.BeginScissorMode(colBounds)
		cui.DrawBoundedText(FONT, cols[i], colBounds, cui.TEXT_LEFT, tColor, color)
		rl.EndScissorMode()
	}
	rl.EndScissorMode()
}

func DrawCurrentSong(bounds *rl.Rectangle, player MusicPlayer) {
	if player.CurrentMusic == nil {
		return
	}
	volumeAndTimeBounds := cui.CutTop(bounds, FONT.Size+4)
	width := volumeAndTimeBounds.Width / 3
	timeBounds := cui.CutLeft(&volumeAndTimeBounds, width)
	volumeBounds := cui.CutLeft(&volumeAndTimeBounds, width)
	modeBounds := volumeAndTimeBounds
	cui.DrawBoxEx(volumeBounds, cui.BoxTheme{1, rl.White, rl.White, rl.Black})
	volume := volumeBounds
	volume.Width *= player.Volume
	cui.DrawBoxEx(volumeBounds, cui.BoxTheme{1, rl.White, rl.White, rl.Black})
	cui.DrawBoxEx(volume, cui.BoxTheme{1, rl.White, rl.White, rl.DarkGreen})
	cui.DrawBoundedText(FONT, "VOLUME", volumeBounds, cui.TEXT_CENTER, rl.White, rl.Black)

	percent := player.TimePlayed / player.TimeLength
	elapsed := timeBounds
	elapsed.Width *= percent
	cui.DrawBoxEx(timeBounds, cui.BoxTheme{1, rl.White, rl.White, rl.Black})
	cui.DrawBoxEx(elapsed, cui.BoxTheme{1, rl.White, rl.White, cui.DarkYellow})
	elapsedText := "no song"
	if player.State == PLAYING {
		timePlayed := FormatDuration(player.TimePlayed)
		elapsedText = fmt.Sprintf("%s/%s", timePlayed, player.CurrentMusic.DurationStr)
	}
	cui.DrawBoundedText(FONT, elapsedText, timeBounds, cui.TEXT_CENTER, rl.White, rl.Black)

	cui.DrawBoxEx(modeBounds, cui.BoxTheme{1, rl.White, rl.White, rl.Black})
	text := "NORMAL"
	switch player.PlayOrder {
	case PLAYORDER_NORMAL:
		text = "NORMAL"
	case PLAYORDER_RANDOM:
		text = "RANDOM"
	case PLAYORDER_RANDOM_NO_REPEAT:
		text = "RANDOM_NO_REPEAT"
	}
	cui.DrawBoundedText(FONT, text, modeBounds, cui.TEXT_CENTER, rl.White, rl.Black)

	if player.RaylibMusic != nil {
		songBounds := cui.CutTop(bounds, FONT.Size+FONT.LineSpacing)
		cui.DrawBoxEx(songBounds, cui.BoxTheme{1, rl.White, rl.White, rl.Black})
		text2 := fmt.Sprintf("%s - %s", player.CurrentMusic.Artist, player.CurrentMusic.Title)
		cui.DrawBoundedText(FONT, text2, songBounds, cui.TEXT_LEFT, rl.White, rl.Black)
	}

}

func DrawFileInfo(file *MusicFile, wrappedFilepath string, wrappedLines int, bounds rl.Rectangle) {
	if file == nil {
		return
	}
	cui.BeginScissorMode(bounds)
	lineHeight := FONT.LineSpacing + FONT.Size
	fileInfoBoundsPadded := cui.PadEx(bounds, FONT.WhiteSpaceWidth*2, FONT.Size/2)
	pos := rl.Vector2{fileInfoBoundsPadded.X, fileInfoBoundsPadded.Y}
	cui.DrawBox(bounds, 1, rl.Black, rl.White)
	cui.DrawText(FONT, pos, wrappedFilepath, rl.White)
	pos.Y += lineHeight * float32(wrappedLines)
	cui.DrawText(FONT, pos, fmt.Sprintf("Artist: %s", file.Artist), rl.White)
	pos.Y += lineHeight
	cui.DrawText(FONT, pos, fmt.Sprintf("Album: %s", file.Album), rl.White)
	pos.Y += lineHeight
	cui.DrawText(FONT, pos, fmt.Sprintf("Title: %s", file.Title), rl.White)
	rl.EndScissorMode()
}

type MusicState int

const (
	STOPPED MusicState = iota
	PLAYING
	PAUSED
)

const (
	STOP = iota + cui.LAST_ACTION
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

var MUSICLIST_KEYBINDINGS = []cui.KeyBind{
	{cui.CLICK, rl.KeyEnter},
	{cui.CANCEL, rl.KeyEscape},
	{PLAY_PAUSE, cui.MOD_CTRL | rl.KeyEnter},
	{STOP, cui.MOD_CTRL | rl.KeySpace},
	{NEXT, cui.MOD_CTRL | rl.KeyRight},
	{PREVIOUS, cui.MOD_CTRL | rl.KeyLeft},
	{FORWARD, rl.KeyRight},
	{REWIND, rl.KeyLeft},
	{VOLUME_DOWN, cui.MOD_CTRL | rl.KeyDown},
	{VOLUME_UP, cui.MOD_CTRL | rl.KeyUp},
	{NEXT_RANDOM, cui.MOD_CTRL | rl.KeyR},
	{NEXT_RANDOM_NO_REPEAT, cui.MOD_CTRL | cui.MOD_SHIFT | rl.KeyR},
	{CHANGE_PLAYORDER_NORMAL, rl.KeyOne},
	{CHANGE_PLAYORDER_RANDOM, rl.KeyTwo},
	{CHANGE_PLAYORDER_RANDOM_NO_REPEAT, rl.KeyThree},
	{DELETE, cui.MOD_CTRL | rl.KeyDelete},
	{COPY_FULL_FILE_PATH, cui.MOD_CTRL | rl.KeyC},
	{REFRESH, rl.KeyF5},
}

func Input(action cui.Action, scroll *cui.Scroll[*MusicFile]) error {
	if action != cui.NONE {
		scroll.Dirty = true
	}
	switch action {
	case cui.CLICK:
		err := PlaySong(&PLAYER, scroll)
		if err != nil {
			return err
		}
	case cui.CANCEL:
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
		cui.CenterCursor(scroll)
	case PREVIOUS:
		index := PrevSong(&PLAYER, scroll)
		scroll.Cursor = index
		cui.CenterCursor(scroll)
	case FORWARD:
		ForwardSong(&PLAYER)
	case REWIND:
		RewindSong(&PLAYER)
	case NEXT_RANDOM_NO_REPEAT:
		index := NextRandomNoRepeatSong(&PLAYER)
		scroll.Cursor = index
		cui.CenterCursor(scroll)
	case NEXT_RANDOM:
		index := NextRandomSong(&PLAYER)
		scroll.Cursor = index
		cui.CenterCursor(scroll)
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

func PlaySong(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) error {
	song := cui.GetHighlightedScrollItem(*scroll, player.Playlist)
	err := playSongAt(player, song.Index)
	if err != nil {
		return err
	}
	if cui.IsFilterActive(*scroll) {
		foundIdx := -1
		for i, idx := range scroll.FilterIndexes {
			if idx == song.Index {
				foundIdx = i
				break
			}
		}
		if foundIdx > 0 {
			scroll.FilterCursor = foundIdx
			cui.CenterCursor(scroll)
		}
	} else {
		scroll.Cursor = song.Index
		cui.CenterCursor(scroll)
	}
	return nil
}
func PlayOrPauseSong(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) error {
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
func UpdatePlayer(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) error {
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
			cui.CenterCursor(scroll)
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
func PrevSong(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) (_index int) {
	if len(player.PlayedSongs) == 0 {
		return
	}
	if player.RaylibMusic != nil {
		rl.StopMusicStream(*player.RaylibMusic)
	}
	nextIndex := 0
	if player.PlayOrder == PLAYORDER_NORMAL {
		nextIndex = cui.Clamp(scroll.Cursor-1, 0, len(player.Playlist))
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
	player.Volume = cui.Min(player.Volume+0.05, 1)
	if player.RaylibMusic != nil {
		rl.SetMusicVolume(*player.RaylibMusic, player.Volume)
	}
}
func VolumeDown(player *MusicPlayer) {
	player.Volume = cui.Max(player.Volume-0.05, 0)
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
	newTime := cui.Min(currentTime+4, maxTime)
	rl.SeekMusicStream(*player.RaylibMusic, newTime)
}
func RewindSong(player *MusicPlayer) {
	if player.RaylibMusic == nil {
		return
	}
	currentTime := rl.GetMusicTimePlayed(*player.RaylibMusic)
	newTime := cui.Max(currentTime-4, 0.01)
	rl.SeekMusicStream(*player.RaylibMusic, newTime)
}
func DeleteSongs(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) error {
	//todo: delete multiple songs, depending on what's selected
	//needs a confirmation modal...maybe
	song := cui.GetHighlightedScrollItem(*scroll, player.Playlist)
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
	scroll.Cursor = cui.Clamp(scroll.Cursor, 0, len(player.Playlist)-1)
	if len(scroll.FilterItems) != 0 {
		scroll.FilterCursor = cui.Clamp(scroll.FilterCursor, 0, len(scroll.FilterItems)-1)
	}
	return nil
}
func RefreshPlaylist(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) error {
	musicfiles, headerCols, err := LoadMusicFiles(player.PlaylistPath)
	if err != nil {
		return err
	}
	player.HeaderCols = headerCols
	player.Playlist = musicfiles
	//scroll.Cursor = 0 //TODO: cursor should behave better
	return nil
}

func SongFilepathToClipboard(player *MusicPlayer, scroll *cui.Scroll[*MusicFile]) {
	song := cui.GetHighlightedScrollItem(*scroll, player.Playlist)
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

func DecideFileColor(selected, highlighted bool, mFile *MusicFile) cui.BoxTheme {
	t := cui.BoxTheme{}
	file := mFile.File
	text1, text2, background := FileTypeColors(file.Ext)
	if highlighted {
		if selected {
			t.Text = rl.Black
			t.Bg = rl.Gold
		} else {
			t.Text = text2
			t.Bg = text1
		}
	} else {
		if selected {
			t.Text = rl.Black
			t.Bg = rl.Yellow
		} else {
			t.Text = text1
			t.Bg = background
		}
	}
	return t
}

func FileTypeColors(ext string) (text1, text2, background rl.Color) {
	switch ext {
	case ".mp3":
		return rl.White, rl.Black, rl.Black
	case ".wav":
		return rl.DarkBlue, rl.Black, rl.Black
	case ".flac":
		return rl.Purple, rl.White, rl.Black
	case ".vgz":
		return rl.DarkGreen, rl.White, rl.Black
	case ".ogg":
		return cui.DarkRed, rl.White, rl.Black
	case ".webm":
		return rl.DarkGray, rl.White, rl.Black
	case ".mp4":
		return rl.Brown, rl.White, rl.Black
	case ".spc":
		return rl.Beige, rl.White, rl.Black
	default:
		return rl.White, rl.Black, rl.Black
	}
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

func ReadFolder(dirPath string) ([]File, error) {
	path, err := ExpandHome(dirPath)
	if err != nil {
		return nil, err
	}
	dirFiles, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0)
	for _, dirEntry := range dirFiles {
		fileInfo, err := os.Lstat(filepath.Join(dirPath, dirEntry.Name()))
		if err != nil {
			fmt.Printf("error: %+v\n", err)
			continue
		}
		fullPath := filepath.Join(dirPath, fileInfo.Name())
		newFile := *NewFile(fullPath, fileInfo)
		files = append(files, newFile)
	}
	return files, nil
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

func SeparateFilenameAndExt(path string) (_filename, _ext string) {
	base := filepath.Base(path)
	index := strings.LastIndex(base, ".")
	if index < 0 {
		return base, ""
	}
	return base[:index], base[index:]
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
