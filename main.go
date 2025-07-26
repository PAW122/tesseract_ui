package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/sqweek/dialog"
)

type Box struct {
	Char rune
	Rect rl.Rectangle
}

func main() {
	var selectedIndex int = -1

	var imagePath string
	if len(os.Args) < 2 {
		// Brak argumentu – zapytaj użytkownika o plik
		path, err := dialog.File().Filter("Images", "png", "jpg", "jpeg").Title("Wybierz obraz").Load()
		if err != nil {
			fmt.Println("Nie wybrano pliku:", err)
			return
		}

		if path == "" {
			fmt.Println("Nie wybrano pliku.")
			return
		}
		imagePath = path
	} else {
		imagePath = os.Args[1]
	}

	img := rl.LoadImage(imagePath)
	if img.Data == nil {
		panic("Nie można załadować obrazu.")
	}

	width := img.Width
	height := img.Height

	// Ogranicz okno do 1280x720 jeśli obrazek jest za duży
	const maxW, maxH = 1400, 900 // zwiększamy skalę bazową

	scale := float32(1.0)
	if width > maxW || height > maxH {
		scaleX := float32(maxW) / float32(width)
		scaleY := float32(maxH) / float32(height)
		if scaleX < scaleY {
			scale = scaleX
		} else {
			scale = scaleY
		}
	}

	const panelWidth = 200
	windowWidth := int32(1600)
	windowHeight := int32(900)

	rl.InitWindow(windowWidth, windowHeight, "Box Annotator")

	rl.SetTargetFPS(60)

	texture := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)

	defer rl.UnloadTexture(texture)
	defer rl.CloseWindow()

	var boxes []Box
	var dragging bool
	var dragOffset rl.Vector2

	boxFilePath := imagePath + ".box"
	if _, err := os.Stat(boxFilePath); err == nil {
		boxes = loadBoxFile(boxFilePath, height)
		fmt.Println("Załadowano istniejący plik .box")
	}

	var activeField string = ""
	var inputBuffer string = ""

	cameraOffset := rl.NewVector2(0, 0)
	var panStart rl.Vector2
	var panActive bool

	var resizing bool
	// var resizeOffset rl.Vector2
	const resizeHandleSize = 12

	scrollOffset := float32(0)

	for !rl.WindowShouldClose() {
		mouse := rl.GetMousePosition()

		scrollDelta := rl.GetMouseWheelMove()
		if scrollDelta != 0 {
			scrollOffset -= scrollDelta * 20
			// ograniczenie (opcjonalnie)
			if scrollOffset < 0 {
				scrollOffset = 0
			}
		}

		// przesuwanie kamery (obrazu) prawym przyciskiem myszy
		if rl.IsMouseButtonPressed(rl.MouseRightButton) {
			panStart = mouse
			panActive = true
		}
		if rl.IsMouseButtonDown(rl.MouseRightButton) && panActive {
			delta := rl.Vector2Subtract(mouse, panStart)
			cameraOffset = rl.Vector2Add(cameraOffset, delta)
			panStart = mouse
		}
		if rl.IsMouseButtonReleased(rl.MouseRightButton) {
			panActive = false
		}

		if rl.IsKeyPressed(rl.KeyKpAdd) || rl.IsKeyPressed(rl.KeyEqual) {
			scale += 0.1
		}
		if rl.IsKeyPressed(rl.KeyKpSubtract) || rl.IsKeyPressed(rl.KeyMinus) {
			if scale > 0.2 {
				scale -= 0.1
			}
		}

		if rl.IsKeyPressed(rl.KeyR) {
			boxes = loadBoxFile(boxFilePath, height)
			fmt.Println("Ponownie załadowano plik .box")
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		// 1. Rysuj obrazek
		rl.DrawTextureEx(texture, cameraOffset, 0, scale, rl.White)

		// 2. Drag & drop zaznaczonego boxa
		hoveringResizeHandle := false

		if selectedIndex >= 0 && selectedIndex < len(boxes) {
			box := &boxes[selectedIndex]
			scaledBox := scaleRect(box.Rect, scale)
			scaledBox.X += cameraOffset.X
			scaledBox.Y += cameraOffset.Y

			// RESIZE: wykrywanie kliknięcia uchwytu
			handle := rl.NewRectangle(
				scaledBox.X+scaledBox.Width-resizeHandleSize,
				scaledBox.Y+scaledBox.Height-resizeHandleSize,
				resizeHandleSize,
				resizeHandleSize,
			)

			if rl.CheckCollisionPointRec(mouse, handle) {
				hoveringResizeHandle = true
			}

			if rl.IsMouseButtonPressed(rl.MouseLeftButton) && hoveringResizeHandle {
				resizing = true
			}

			if resizing && rl.IsMouseButtonDown(rl.MouseLeftButton) {
				newW := (mouse.X - cameraOffset.X - box.Rect.X*scale) / scale
				newH := (mouse.Y - cameraOffset.Y - box.Rect.Y*scale) / scale
				if newW >= 5 {
					box.Rect.Width = newW
				}
				if newH >= 5 {
					box.Rect.Height = newH
				}
			}

			if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
				resizing = false
			}

			// DRAGGING tylko jeśli NIE resize’ujemy
			if !resizing {
				if rl.CheckCollisionPointRec(mouse, scaledBox) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
					dragging = true
					dragOffset = rl.Vector2Subtract(mouse, rl.NewVector2(scaledBox.X, scaledBox.Y))
				}
				if dragging && rl.IsMouseButtonDown(rl.MouseLeftButton) {
					newPos := rl.Vector2{
						X: (mouse.X - cameraOffset.X - dragOffset.X) / scale,
						Y: (mouse.Y - cameraOffset.Y - dragOffset.Y) / scale,
					}
					box.Rect.X = newPos.X
					box.Rect.Y = newPos.Y
				}
				if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
					dragging = false
				}
			}
		}

		// Resizing box (dolny-prawy róg)
		if !dragging && selectedIndex >= 0 && selectedIndex < len(boxes) {
			box := &boxes[selectedIndex]
			r := scaleRect(box.Rect, scale)
			r.X += cameraOffset.X
			r.Y += cameraOffset.Y

			handle := rl.NewRectangle(
				r.X+r.Width-resizeHandleSize,
				r.Y+r.Height-resizeHandleSize,
				resizeHandleSize,
				resizeHandleSize,
			)

			if rl.CheckCollisionPointRec(mouse, handle) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
				resizing = true
			}
			if resizing && rl.IsMouseButtonDown(rl.MouseLeftButton) {
				newW := (mouse.X - cameraOffset.X - box.Rect.X*scale) / scale
				newH := (mouse.Y - cameraOffset.Y - box.Rect.Y*scale) / scale
				if newW >= 5 {
					box.Rect.Width = newW
				}
				if newH >= 5 {
					box.Rect.Height = newH
				}
			}
			if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
				resizing = false
			}
		}

		// 3. Rysowanie boxów
		for i, box := range boxes {
			r := scaleRect(box.Rect, scale)
			r.X += cameraOffset.X
			r.Y += cameraOffset.Y

			color := rl.Blue
			if i == selectedIndex {
				color = rl.Red
			}
			rl.DrawRectangleLinesEx(r, 2, color)
			rl.DrawText(string(box.Char), int32(r.X)+2, int32(r.Y)+2, 16, rl.Black)

			if i == selectedIndex {
				// r to już przeskalowany i przesunięty box
				rl.DrawRectangle(
					int32(r.X+r.Width-resizeHandleSize),
					int32(r.Y+r.Height-resizeHandleSize),
					resizeHandleSize,
					resizeHandleSize,
					rl.DarkGray,
				)
			}

		}

		// 4. Prawy panel
		panelX := float32(rl.GetScreenWidth()) - 200
		rl.DrawRectangle(int32(panelX), 0, 200, int32(rl.GetScreenHeight()), rl.LightGray)

		// Przycisk: NEW BOX
		newBoxBtn := rl.NewRectangle(panelX+20, 20, 160, 30)
		rl.DrawRectangleRec(newBoxBtn, rl.Gray)
		rl.DrawText("New Box", int32(newBoxBtn.X+40), int32(newBoxBtn.Y+7), 16, rl.Black)
		if rl.CheckCollisionPointRec(mouse, newBoxBtn) && rl.IsMouseButtonReleased(rl.MouseLeftButton) {
			boxes = append(boxes, Box{
				Char: '?',
				Rect: rl.Rectangle{
					X:     float32(width)/2 - 50,
					Y:     float32(height)/2 - 20,
					Width: 100, Height: 40,
				},
			})
		}

		// Przycisk: SAVE
		saveBtn := rl.NewRectangle(panelX+20, 60, 160, 30)
		rl.DrawRectangleRec(saveBtn, rl.Gray)
		rl.DrawText("Save", int32(saveBtn.X+60), int32(saveBtn.Y+7), 16, rl.Black)
		if rl.CheckCollisionPointRec(mouse, saveBtn) && rl.IsMouseButtonReleased(rl.MouseLeftButton) {
			saveBoxFile(imagePath, boxes, height)
			fmt.Println("Zapisano .box!")
		}

		// Przycisk: DELETE
		if selectedIndex >= 0 && selectedIndex < len(boxes) {
			delBtn := rl.NewRectangle(panelX+20, 100, 160, 30)
			rl.DrawRectangleRec(delBtn, rl.Gray)
			rl.DrawText("Delete", int32(delBtn.X+50), int32(delBtn.Y+7), 16, rl.Black)
			if rl.CheckCollisionPointRec(mouse, delBtn) && rl.IsMouseButtonReleased(rl.MouseLeftButton) {
				boxes = append(boxes[:selectedIndex], boxes[selectedIndex+1:]...)
				selectedIndex = -1
			}
		}

		// Przycisk: DUPLICATE
		dupBtn := rl.NewRectangle(panelX+20, 140, 160, 30)
		rl.DrawRectangleRec(dupBtn, rl.Gray)
		rl.DrawText("Duplicate", int32(dupBtn.X+40), int32(dupBtn.Y+7), 16, rl.Black)
		if rl.CheckCollisionPointRec(mouse, dupBtn) && rl.IsMouseButtonReleased(rl.MouseLeftButton) {
			if selectedIndex >= 0 && selectedIndex < len(boxes) {
				orig := boxes[selectedIndex]
				newBox := Box{
					Char: orig.Char,
					Rect: rl.Rectangle{
						X:      orig.Rect.X + 10,
						Y:      orig.Rect.Y + 10,
						Width:  orig.Rect.Width,
						Height: orig.Rect.Height,
					},
				}
				boxes = append(boxes, newBox)
				selectedIndex = len(boxes) - 1
			}
		}

		// napis z skalowania
		rl.DrawText(fmt.Sprintf("Zoom: %.1fx", scale), 10, 10, 18, rl.Black)

		// Edycja danych boxa
		rl.DrawText("Boxy:", int32(panelX+10), 175, 20, rl.DarkGray)

		listStartY := float32(200)
		entryHeight := float32(20)
		for i, box := range boxes {
			entryY := listStartY + float32(i)*entryHeight - scrollOffset

			// pomiń jeśli poza ekranem (opcjonalne przyspieszenie)
			listVisibleStartY := float32(200)                      // dolna granica widoczności
			listVisibleEndY := float32(rl.GetScreenHeight()) - 220 // górna granica, zostawia miejsce na edycję

			if entryY < listVisibleStartY || entryY > listVisibleEndY {
				continue
			}

			label := fmt.Sprintf("[%d] '%c'", i, box.Char)
			color := rl.Black
			if i == selectedIndex {
				color = rl.Red
			}
			rl.DrawText(label, int32(panelX+10), int32(entryY), 16, color)

			entryRect := rl.NewRectangle(panelX, entryY, 200, entryHeight)
			if rl.CheckCollisionPointRec(mouse, entryRect) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
				selectedIndex = i
			}
		}

		// Edytor wartości zaznaczonego boxa
		if selectedIndex >= 0 && selectedIndex < len(boxes) {
			fieldW := float32(140)
			lineH := float32(20)
			editY := float32(rl.GetScreenHeight()) - 110

			fields := []struct {
				Label string
				Value float32
				Y     float32
			}{
				{"x", boxes[selectedIndex].Rect.X, editY},
				{"y", boxes[selectedIndex].Rect.Y, editY + lineH},
				{"w", boxes[selectedIndex].Rect.Width, editY + 2*lineH},
				{"h", boxes[selectedIndex].Rect.Height, editY + 3*lineH},
			}

			for _, field := range fields {
				isActive := activeField == field.Label
				fieldRect := rl.NewRectangle(panelX+30, field.Y, fieldW, lineH)

				// Tło pola
				if isActive {
					rl.DrawRectangleRec(fieldRect, rl.SkyBlue)
				} else {
					rl.DrawRectangleRec(fieldRect, rl.LightGray)
				}

				// Etykieta
				rl.DrawText(fmt.Sprintf("%s:", field.Label), int32(panelX+10), int32(field.Y+2), 16, rl.Black)

				// Tekst w polu
				display := fmt.Sprintf("%.0f", field.Value)
				if isActive {
					display = inputBuffer + "_"
				}
				rl.DrawText(display, int32(fieldRect.X+4), int32(fieldRect.Y+2), 16, rl.Black)

				// Aktywacja pola kliknięciem
				if rl.CheckCollisionPointRec(mouse, fieldRect) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
					activeField = field.Label
					inputBuffer = ""
				}
			}

		}

		// 6. Wprowadzenie litery
		if rl.IsKeyPressed(rl.GetKeyPressed()) && selectedIndex >= 0 && selectedIndex < len(boxes) {
			c := rl.GetCharPressed()
			if c != 0 {
				boxes[selectedIndex].Char = rune(c)
			}
		}

		rl.EndDrawing()

		// Obsługa edycji tekstu
		if activeField != "" {
			key := rl.GetCharPressed()
			for key > 0 {
				if key >= 48 && key <= 57 { // tylko cyfry
					inputBuffer += string(rune(key))
				}
				key = rl.GetCharPressed()
			}

			// Backspace
			if rl.IsKeyPressed(rl.KeyBackspace) && len(inputBuffer) > 0 {
				inputBuffer = inputBuffer[:len(inputBuffer)-1]
			}

			// Enter = zatwierdź
			if rl.IsKeyPressed(rl.KeyEnter) && selectedIndex >= 0 {
				val, err := strconv.Atoi(inputBuffer)
				if err == nil {
					switch activeField {
					case "x":
						boxes[selectedIndex].Rect.X = float32(val)
					case "y":
						boxes[selectedIndex].Rect.Y = float32(val)
					case "w":
						boxes[selectedIndex].Rect.Width = float32(val)
					case "h":
						boxes[selectedIndex].Rect.Height = float32(val)
					}
				}
				activeField = ""
				inputBuffer = ""
			}
		}

	}

}

func scaleRect(r rl.Rectangle, s float32) rl.Rectangle {
	return rl.Rectangle{
		X:      r.X * s,
		Y:      r.Y * s,
		Width:  r.Width * s,
		Height: r.Height * s,
	}
}

func saveBoxFile(imagePath string, boxes []Box, imageHeight int32) {
	name := imagePath + ".box"
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	for _, box := range boxes {
		x1 := int(box.Rect.X)
		y1 := int(box.Rect.Y)
		x2 := x1 + int(box.Rect.Width)
		y2 := y1 + int(box.Rect.Height)

		// Konwersja osi Y
		y1t := int(imageHeight) - y1
		y2t := int(imageHeight) - y2
		if y2t > y1t {
			y1t, y2t = y2t, y1t
		}

		line := fmt.Sprintf("%c %d %d %d %d 0\n", box.Char, x1, y2t, x2, y1t)
		f.WriteString(line)
	}
}

func loadBoxFile(path string, imageHeight int32) []Box {
	f, err := os.Open(path)
	if err != nil {
		fmt.Println("Nie można otworzyć pliku .box:", err)
		return nil
	}
	defer f.Close()

	var boxes []Box
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ch rune
		var x1, y1, x2, y2 int
		line := scanner.Text()
		_, err := fmt.Sscanf(line, "%c %d %d %d %d", &ch, &x1, &y1, &x2, &y2)
		if err != nil {
			fmt.Println("Nieprawidłowy format linii:", line)
			continue
		}

		// Odwrócenie Y z powrotem
		box := Box{
			Char: ch,
			Rect: rl.Rectangle{
				X:      float32(x1),
				Y:      float32(imageHeight - int32(y2)),
				Width:  float32(x2 - x1),
				Height: float32(y2 - y1),
			},
		}
		boxes = append(boxes, box)
	}
	return boxes
}

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func abs(a float32) float32 {
	if a < 0 {
		return -a
	}
	return a
}
