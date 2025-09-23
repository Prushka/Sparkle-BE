package sup

import (
	"image"
)

// isRowTransparent checks if a given horizontal row in an image is entirely transparent.
func isRowTransparent(img image.Image, y int) bool {
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		_, _, _, a := img.At(x, y).RGBA()
		if a != 0 {
			return false
		}
	}
	return true
}

// TrimTransparentRows trims consecutive fully transparent horizontal lines in an image.
// If more than 10 consecutive transparent lines are found, they are trimmed down to 10.
func TrimTransparentRows(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 1. Identify all transparent rows
	transparentRows := make([]bool, height)
	for y := 0; y < height; y++ {
		transparentRows[y] = isRowTransparent(img, bounds.Min.Y+y)
	}

	// 2. Determine the height of the new image
	newHeight := 0
	consecutiveTransparentCount := 0
	for _, isTransparent := range transparentRows {
		if isTransparent {
			consecutiveTransparentCount++
		} else {
			consecutiveTransparentCount = 0
		}

		if consecutiveTransparentCount <= 10 {
			newHeight++
		}
	}

	// If the height hasn't changed, return the original image
	if newHeight == height {
		return img
	}

	// 3. Create a new image with the calculated height
	newImg := image.NewRGBA(image.Rect(0, 0, width, newHeight))

	// 4. Copy the relevant rows to the new image
	destY := 0
	consecutiveTransparentCount = 0
	for y := 0; y < height; y++ {
		isTransparent := transparentRows[y]
		if isTransparent {
			consecutiveTransparentCount++
		} else {
			consecutiveTransparentCount = 0
		}

		if consecutiveTransparentCount <= 10 {
			// Copy the row
			for x := 0; x < width; x++ {
				newImg.Set(x, destY, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
			destY++
		}
	}

	return newImg
}
